package private

import (
	"net/http"
	"playbook-dispatcher/internal/api/connectors"
	"playbook-dispatcher/internal/api/connectors/inventory"
	"playbook-dispatcher/internal/api/connectors/sources"
	"playbook-dispatcher/internal/api/controllers/public"
	"playbook-dispatcher/internal/common/utils"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type rhcSatellite struct {
	SatelliteInstanceID      string
	SatelliteOrgID           string
	SatelliteVersion         string
	Hosts                    []string
	SourceID                 string
	RhcClientID              *string
	SourceAvailabilityStatus *string
}

func (this *controllers) ApiInternalHighlevelConnectionStatus(ctx echo.Context) error {
	var input HostsWithOrgId
	satelliteResponses := []RecipientWithConnectionInfo{}
	directConnectedResponses := []RecipientWithConnectionInfo{}
	noRHCResponses := []RecipientWithConnectionInfo{}

	err := utils.ReadRequestBody(ctx, &input)
	if err != nil {
		utils.GetLogFromEcho(ctx).Error(err)
		return ctx.NoContent(http.StatusBadRequest)
	}

	if len(input.Hosts) > 50 {
		utils.GetLogFromEcho(ctx).Infow("More than 50 hosts requested")

		return ctx.JSON(http.StatusBadRequest, map[string]string{
			"message": "maximum input length exceeded",
		})
	}

	hostConnectorDetails, err := this.inventoryConnectorClient.GetHostConnectionDetails(
		ctx.Request().Context(),
		input.Hosts,
		this.config.GetString("inventory.connector.ordered.by"),
		this.config.GetString("inventory.connector.ordered.how"),
		this.config.GetInt("inventory.connector.limit"),
		this.config.GetInt("inventory.connector.offset"),
	)

	utils.GetLogFromEcho(ctx).Infow("returned from inventory", "data", hostConnectorDetails, "error", err)

	if err != nil {
		utils.GetLogFromEcho(ctx).Error(err)
		return ctx.NoContent(http.StatusBadRequest)
	}

	if len(hostConnectorDetails) == 0 {
		utils.GetLogFromEcho(ctx).Infow("host(s) not found in inventory", "data", noRHCResponses)
		return ctx.JSON(http.StatusOK, noRHCResponses)
	}

	satellite, directConnected, noRhc := sortHostsByRecipient(hostConnectorDetails)

	// Return noRHC If no Satellite or Direct Connected hosts exist
	if noRhc != nil {
		noRHCResponses = []RecipientWithConnectionInfo{getRHCStatus(noRhc, input.OrgId)}
	}

	if satellite == nil && directConnected == nil {
		utils.GetLogFromEcho(ctx).Infow("no satellite or direct connected systems", "data", noRHCResponses)
		return ctx.JSON(http.StatusOK, noRHCResponses)
	}

	if len(satellite) > 0 {
		satelliteResponses, err = getSatelliteStatus(ctx, this.cloudConnectorClient, this.sourcesConnectorClient, input.OrgId, satellite)

		utils.GetLogFromEcho(ctx).Infow("satellite status", "data", satelliteResponses, "error", err)

		if err != nil {
			utils.GetLogFromEcho(ctx).Errorf("Error retrieving Satellite status: %s", err)
		}
	}

	if len(directConnected) > 0 {
		directConnectedResponses, err = getDirectConnectStatus(ctx, this.cloudConnectorClient, input.OrgId, directConnected)

		utils.GetLogFromEcho(ctx).Infow("direct connect status", "data", directConnectedResponses, "error", err)

		if err != nil {
			utils.GetLogFromEcho(ctx).Errorf("Error retrieving Direct Connect status: %s", err)
		}
	}

	highLevelStatus := HighLevelRecipientStatus(concatResponses(satelliteResponses, directConnectedResponses, noRHCResponses))
	utils.GetLogFromEcho(ctx).Infow("returning high level status", "data", highLevelStatus)
	return ctx.JSON(http.StatusOK, highLevelStatus)
}

func sortHostsByRecipient(details []inventory.HostDetails) (satelliteDetails []inventory.HostDetails, directConnectedDetails []inventory.HostDetails, noRhc []inventory.HostDetails) {
	var satelliteConnectedHosts []inventory.HostDetails
	var directConnectedHosts []inventory.HostDetails
	var hostsNotConnected []inventory.HostDetails

	for _, host := range details {
		switch {
		case host.SatelliteInstanceID != nil:
			satelliteConnectedHosts = append(satelliteConnectedHosts, host) // If satellite_instance_id exitsts Satellite host
		case host.RHCClientID != nil:
			directConnectedHosts = append(directConnectedHosts, host) // if rhc_client_id exists in inventory facts host is direct connect
		default:
			hostsNotConnected = append(hostsNotConnected, host)
		}
	}

	return satelliteConnectedHosts, directConnectedHosts, hostsNotConnected
}

func formatConnectionResponse(satID *string, satOrgID *string, rhcClientID *string, orgID OrgId, hosts []string, recipientType string, status string) RecipientWithConnectionInfo {
	formatedHosts := make([]HostId, len(hosts))
	var formatedSatID SatelliteId
	var formatedSatOrgID SatelliteOrgId
	var formatedRHCClientID public.RunRecipient

	if satID != nil {
		formatedSatID = SatelliteId(*satID)
	}

	if satOrgID != nil {
		formatedSatOrgID = SatelliteOrgId(*satOrgID)
	}

	if rhcClientID != nil {
		rhcClientUUID, _ := uuid.Parse(*rhcClientID)
		formatedRHCClientID = public.RunRecipient(rhcClientUUID)
	}

	for i, host := range hosts {
		formatedHosts[i] = HostId(host)
	}

	connectionInfo := RecipientWithConnectionInfo{
		OrgId:         orgID,
		Recipient:     formatedRHCClientID,
		RecipientType: RecipientType(recipientType),
		SatId:         formatedSatID,
		SatOrgId:      formatedSatOrgID,
		Status:        RecipientWithConnectionInfoStatus(status),
		Systems:       formatedHosts,
	}

	return connectionInfo
}

func getDirectConnectStatus(ctx echo.Context, client connectors.CloudConnectorClient, orgId OrgId, hostDetails []inventory.HostDetails) ([]RecipientWithConnectionInfo, error) {
	responses := []RecipientWithConnectionInfo{}
	for _, host := range hostDetails {
		status, err := client.GetConnectionStatus(ctx.Request().Context(), string(orgId), *host.RHCClientID)

		if err != nil {
			utils.GetLogFromEcho(ctx).Error(err)
			return nil, ctx.NoContent(http.StatusInternalServerError)
		}

		var connectionStatus string
		if status == connectors.Connected {
			connectionStatus = "connected"
		} else {
			connectionStatus = "disconnected"
		}

		responses = append(responses, formatConnectionResponse(nil, nil, host.RHCClientID, orgId, []string{host.ID}, string(DirectConnect), connectionStatus))
	}

	return responses, nil
}

func getSatelliteStatus(ctx echo.Context, client connectors.CloudConnectorClient, sourceClient sources.SourcesConnector, orgId OrgId, hostDetails []inventory.HostDetails) ([]RecipientWithConnectionInfo, error) {
	hostsGroupedBySatellite := groupHostsBySatellite(hostDetails)

	hostsGroupedBySatellite = getSourceInfo(ctx, hostsGroupedBySatellite, sourceClient)

	responses, err := createSatelliteConnectionResponses(ctx, hostsGroupedBySatellite, client, orgId)
	if err != nil {
		utils.GetLogFromEcho(ctx).Error("error occured creating satellite connection response")
		return nil, ctx.NoContent(http.StatusInternalServerError)
	}

	return responses, nil
}

func groupHostsBySatellite(hostDetails []inventory.HostDetails) map[string]*rhcSatellite {
	hostsGroupedBySatellite := make(map[string]*rhcSatellite)

	for _, host := range hostDetails {
		satInstanceAndOrg := *host.SatelliteInstanceID + *host.SatelliteOrgID
		_, exists := hostsGroupedBySatellite[satInstanceAndOrg]

		if exists {
			hostsGroupedBySatellite[satInstanceAndOrg].Hosts = append(hostsGroupedBySatellite[satInstanceAndOrg].Hosts, host.ID)
		} else {
			satellite := &rhcSatellite{
				SatelliteInstanceID: *host.SatelliteInstanceID,
				SatelliteOrgID:      *host.SatelliteOrgID,
				Hosts:               []string{host.ID},
			}

			if host.SatelliteVersion != nil {
				satellite.SatelliteVersion = *host.SatelliteVersion
			}

			hostsGroupedBySatellite[satInstanceAndOrg] = satellite
		}
	}

	return hostsGroupedBySatellite
}

func getSourceInfo(ctx echo.Context, hostsGroupedBySatellite map[string]*rhcSatellite, sourceClient sources.SourcesConnector) map[string]*rhcSatellite {
	for i, satellite := range hostsGroupedBySatellite {
		result, err := sourceClient.GetSourceConnectionDetails(ctx.Request().Context(), satellite.SatelliteInstanceID)

		if err != nil {
			utils.GetLogFromEcho(ctx).Errorf("Sources data could not be found for SatelliteID %s Error: %s", satellite.SatelliteInstanceID, err)
		} else {
			hostsGroupedBySatellite[i].SourceID = result.ID
			hostsGroupedBySatellite[i].RhcClientID = result.RhcID
			hostsGroupedBySatellite[i].SourceAvailabilityStatus = result.AvailabilityStatus
		}
	}

	return hostsGroupedBySatellite
}

func createSatelliteConnectionResponses(ctx echo.Context, hostsGroupedBySatellite map[string]*rhcSatellite, cloudConnector connectors.CloudConnectorClient, orgId OrgId) ([]RecipientWithConnectionInfo, error) {
	responses := []RecipientWithConnectionInfo{}

	for _, satellite := range hostsGroupedBySatellite {
		if satellite.RhcClientID != nil {
			status, err := cloudConnector.GetConnectionStatus(ctx.Request().Context(), satellite.SatelliteOrgID, *satellite.RhcClientID)
			if err != nil {
				utils.GetLogFromEcho(ctx).Error(err)
				return nil, ctx.NoContent(http.StatusInternalServerError)
			}

			var connectionStatus string
			if status == connectors.Connected {
				connectionStatus = "connected"
			} else {
				connectionStatus = "disconnected"
			}

			responses = append(responses, formatConnectionResponse(&satellite.SatelliteInstanceID, &satellite.SatelliteOrgID, satellite.RhcClientID, orgId, satellite.Hosts, string(Satellite), connectionStatus))
		}
	}

	return responses, nil
}

func getRHCStatus(hostDetails []inventory.HostDetails, orgID OrgId) RecipientWithConnectionInfo {
	hostIDs := make([]string, len(hostDetails))

	for i, host := range hostDetails {
		hostIDs[i] = host.ID
	}

	return formatConnectionResponse(nil, nil, nil, orgID, hostIDs, "none", "rhc_not_configured")
}

func concatResponses(satellite []RecipientWithConnectionInfo, directConnect []RecipientWithConnectionInfo, noRHC []RecipientWithConnectionInfo) []RecipientWithConnectionInfo {
	responses := append(satellite, directConnect...)

	return append(responses, noRHC...)
}

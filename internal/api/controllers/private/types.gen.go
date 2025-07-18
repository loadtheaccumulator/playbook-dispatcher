// Package private provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.5.0 DO NOT EDIT.
package private

import (
	externalRef0 "playbook-dispatcher/internal/api/controllers/public"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Defines values for RecipientType.
const (
	DirectConnect RecipientType = "directConnect"
	None          RecipientType = "none"
	Satellite     RecipientType = "satellite"
)

// Defines values for RecipientWithConnectionInfoStatus.
const (
	Connected        RecipientWithConnectionInfoStatus = "connected"
	Disconnected     RecipientWithConnectionInfoStatus = "disconnected"
	RhcNotConfigured RecipientWithConnectionInfoStatus = "rhc_not_configured"
)

// CancelInputV2 defines model for CancelInputV2.
type CancelInputV2 struct {
	// OrgId Identifies the organization that the given resource belongs to
	OrgId OrgId `json:"org_id"`

	// Principal Username of the user interacting with the service
	Principal Principal `json:"principal"`

	// RunId Unique identifier of a Playbook run
	RunId externalRef0.RunId `json:"run_id"`
}

// Error defines model for Error.
type Error struct {
	// Message Human readable error message
	Message string `json:"message"`
}

// HighLevelRecipientStatus defines model for HighLevelRecipientStatus.
type HighLevelRecipientStatus = []RecipientWithConnectionInfo

// HostId Identifies a record of the Host-Inventory service
type HostId = string

// HostsWithOrgId defines model for HostsWithOrgId.
type HostsWithOrgId struct {
	Hosts []string `json:"hosts"`

	// OrgId Identifies the organization that the given resource belongs to
	OrgId OrgId `json:"org_id"`
}

// OrgId Identifies the organization that the given resource belongs to
type OrgId = string

// Principal Username of the user interacting with the service
type Principal = string

// RecipientConfig recipient-specific configuration options
type RecipientConfig struct {
	// SatId Identifier of the Satellite instance in the uuid v4/v5 format
	SatId *string `json:"sat_id,omitempty"`

	// SatOrgId Identifier of the organization within Satellite
	SatOrgId *string `json:"sat_org_id,omitempty"`
}

// RecipientStatus defines model for RecipientStatus.
type RecipientStatus struct {
	// Connected Indicates whether a connection is established with the recipient
	Connected bool `json:"connected"`

	// OrgId Identifies the organization that the given resource belongs to
	OrgId OrgId `json:"org_id"`

	// Recipient Identifier of the host to which a given Playbook is addressed
	Recipient externalRef0.RunRecipient `json:"recipient"`
}

// RecipientType Identifies the type of recipient [Satellite, Direct Connected, None]
type RecipientType string

// RecipientWithConnectionInfo defines model for RecipientWithConnectionInfo.
type RecipientWithConnectionInfo struct {
	// OrgId Identifies the organization that the given resource belongs to
	OrgId OrgId `json:"org_id"`

	// Recipient Identifier of the host to which a given Playbook is addressed
	Recipient externalRef0.RunRecipient `json:"recipient"`

	// RecipientType Identifies the type of recipient [Satellite, Direct Connected, None]
	RecipientType RecipientType `json:"recipient_type"`

	// SatId Identifier of the Satellite instance in the uuid v4/v5 format
	SatId SatelliteId `json:"sat_id"`

	// SatOrgId Identifier of the organization within Satellite
	SatOrgId SatelliteOrgId `json:"sat_org_id"`

	// Status Indicates the current run status of the recipient
	Status  RecipientWithConnectionInfoStatus `json:"status"`
	Systems []HostId                          `json:"systems"`
}

// RecipientWithConnectionInfoStatus Indicates the current run status of the recipient
type RecipientWithConnectionInfoStatus string

// RecipientWithOrg defines model for RecipientWithOrg.
type RecipientWithOrg struct {
	// OrgId Identifies the organization that the given resource belongs to
	OrgId OrgId `json:"org_id"`

	// Recipient Identifier of the host to which a given Playbook is addressed
	Recipient externalRef0.RunRecipient `json:"recipient"`
}

// RunCanceled defines model for RunCanceled.
type RunCanceled struct {
	// Code status code of the request
	Code int `json:"code"`

	// RunId Unique identifier of a Playbook run
	RunId externalRef0.RunId `json:"run_id"`
}

// RunCreated defines model for RunCreated.
type RunCreated struct {
	// Code status code of the request
	Code int `json:"code"`

	// Id Unique identifier of a Playbook run
	Id *externalRef0.RunId `json:"id,omitempty"`

	// Message Error Message
	Message *string `json:"message,omitempty"`
}

// RunInput defines model for RunInput.
type RunInput struct {
	// Account Identifier of the tenant
	// Deprecated: this property has been marked as deprecated upstream, but no `x-deprecated-reason` was set
	Account externalRef0.Account `json:"account"`

	// Hosts Optionally, information about hosts involved in the Playbook run can be provided.
	// This information is used to pre-allocate run_host resources.
	// Moreover, it can be used to create a connection between a run_host resource and host inventory.
	Hosts *RunInputHosts `json:"hosts,omitempty"`

	// Labels Additional metadata about the Playbook run. Can be used for filtering purposes.
	Labels *externalRef0.Labels `json:"labels,omitempty"`

	// Recipient Identifier of the host to which a given Playbook is addressed
	Recipient externalRef0.RunRecipient `json:"recipient"`

	// Timeout Amount of seconds after which the run is considered failed due to timeout
	Timeout *externalRef0.RunTimeout `json:"timeout,omitempty"`

	// Url URL hosting the Playbook
	Url externalRef0.Url `json:"url"`
}

// RunInputHosts Optionally, information about hosts involved in the Playbook run can be provided.
// This information is used to pre-allocate run_host resources.
// Moreover, it can be used to create a connection between a run_host resource and host inventory.
type RunInputHosts = []struct {
	// AnsibleHost Host name as known to Ansible inventory.
	// Used to identify the host in status reports.
	AnsibleHost *string `json:"ansible_host,omitempty"`

	// InventoryId Inventory id of the given host
	InventoryId *openapi_types.UUID `json:"inventory_id,omitempty"`
}

// RunInputV2 defines model for RunInputV2.
type RunInputV2 struct {
	// Hosts Optionally, information about hosts involved in the Playbook run can be provided.
	// This information is used to pre-allocate run_host resources.
	// Moreover, it can be used to create a connection between a run_host resource and host inventory.
	Hosts *RunInputHosts `json:"hosts,omitempty"`

	// Labels Additional metadata about the Playbook run. Can be used for filtering purposes.
	Labels *externalRef0.Labels `json:"labels,omitempty"`

	// Name Human readable name of the playbook run. Used to present the given playbook run in external systems (Satellite).
	Name externalRef0.PlaybookName `json:"name"`

	// OrgId Identifier of the tenant
	OrgId externalRef0.OrgId `json:"org_id"`

	// Principal Username of the user interacting with the service
	Principal Principal `json:"principal"`

	// Recipient Identifier of the host to which a given Playbook is addressed
	Recipient externalRef0.RunRecipient `json:"recipient"`

	// RecipientConfig recipient-specific configuration options
	RecipientConfig *RecipientConfig `json:"recipient_config,omitempty"`

	// Timeout Amount of seconds after which the run is considered failed due to timeout
	Timeout *externalRef0.RunTimeout `json:"timeout,omitempty"`

	// Url URL hosting the Playbook
	Url externalRef0.Url `json:"url"`

	// WebConsoleUrl URL that points to the section of the web console where the user find more information about the playbook run. The field is optional but highly suggested.
	WebConsoleUrl *externalRef0.WebConsoleUrl `json:"web_console_url,omitempty"`
}

// RunsCanceled defines model for RunsCanceled.
type RunsCanceled = []RunCanceled

// RunsCreated defines model for RunsCreated.
type RunsCreated = []RunCreated

// SatelliteId Identifier of the Satellite instance in the uuid v4/v5 format
type SatelliteId = string

// SatelliteOrgId Identifier of the organization within Satellite
type SatelliteOrgId = string

// Version Version of the API
type Version = string

// BadRequest defines model for BadRequest.
type BadRequest = Error

// ApiInternalRunsCreateJSONBody defines parameters for ApiInternalRunsCreate.
type ApiInternalRunsCreateJSONBody = []RunInput

// ApiInternalV2RunsCancelJSONBody defines parameters for ApiInternalV2RunsCancel.
type ApiInternalV2RunsCancelJSONBody = []CancelInputV2

// ApiInternalV2RunsCreateJSONBody defines parameters for ApiInternalV2RunsCreate.
type ApiInternalV2RunsCreateJSONBody = []RunInputV2

// ApiInternalV2RecipientsStatusJSONBody defines parameters for ApiInternalV2RecipientsStatus.
type ApiInternalV2RecipientsStatusJSONBody = []RecipientWithOrg

// ApiInternalRunsCreateJSONRequestBody defines body for ApiInternalRunsCreate for application/json ContentType.
type ApiInternalRunsCreateJSONRequestBody = ApiInternalRunsCreateJSONBody

// ApiInternalV2RunsCancelJSONRequestBody defines body for ApiInternalV2RunsCancel for application/json ContentType.
type ApiInternalV2RunsCancelJSONRequestBody = ApiInternalV2RunsCancelJSONBody

// ApiInternalHighlevelConnectionStatusJSONRequestBody defines body for ApiInternalHighlevelConnectionStatus for application/json ContentType.
type ApiInternalHighlevelConnectionStatusJSONRequestBody = HostsWithOrgId

// ApiInternalV2RunsCreateJSONRequestBody defines body for ApiInternalV2RunsCreate for application/json ContentType.
type ApiInternalV2RunsCreateJSONRequestBody = ApiInternalV2RunsCreateJSONBody

// ApiInternalV2RecipientsStatusJSONRequestBody defines body for ApiInternalV2RecipientsStatus for application/json ContentType.
type ApiInternalV2RecipientsStatusJSONRequestBody = ApiInternalV2RecipientsStatusJSONBody

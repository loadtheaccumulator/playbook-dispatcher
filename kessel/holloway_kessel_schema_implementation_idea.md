Playbook Dispatcher Kessel Design Idea(s)
Current RBAC Flow
1. Request arrives with identity header
2. Middleware calls RBAC service REST API to fetch all permissions
3. RBAC returns permissions with `resourceDefinitions`

{
  "permission": "playbook-dispatcher:run:read",
  "resourceDefinitions": [
    {
      "attributeFilter": {
        "key": "service",
        "operation": "in",
        "value": ["remediations", "config-manager"]
      }
    }
  ]
}

3. Code extracts `service` attribute values using `GetPredicateValues()`
4. SQL query filters: `WHERE service IN ('remediations', 'config-manager')`

    // rbac
    permissions := middleware.GetPermissions(ctx)
    if allowedServices := rbac.GetPredicateValues(permissions, "service"); len(allowedServices) > 0 {
        queryBuilder.Where("service IN ?", allowedServices)
    }

New Kessel Flow
1. Request arrives with identity header
2. Middleware extracts subject (user) and resource from request
3. Playbook Dispatcher loops through a list of knownServices.
    Calls Kessel API for each to check specific permission,
    Creates the list of allowedServices as it loops.
This falls through and the existing code works as-is from here...
4. SQL query filters: `WHERE service IN ('remediations', 'config-manager')`

// rbac & kessel
if allowedServices := getAllowedServices(ctx); len(allowedServices) > 0 {
        queryBuilder.Where("service IN ?", allowedServices)
}

SIDENOTE: Moving the above rbac calls, GetPermissions() and GetPredicateValues(), into getAllowedServices() along with the kessel logic wrapped by feature flags. Also looking at moving this to the front of the endpoint middleware call, passing allowedServices or returning immediately if empty list.

Current Remediations RBAC role
    {
      "name": "Remediations user",
      "description": "Perform create, read, update, delete operations on any Remediations resource.",
      "system": true,
      "platform_default": true,
      "version": 6,
      "access": [
        {
          "permission": "remediations:remediation:read"
        },
        {
          "permission": "remediations:remediation:write"
        },
        {
          "permission": "playbook-dispatcher:run:read",
          "resourceDefinitions": [
            {
              "attributeFilter": {
                "key": "service",
                "operation": "equal",
                "value": "remediations"
              }
            }
          ]
        }
      ]
    }
Proposed Playbook Dispatcher Schema Option 1
Insert @rbac.add_v1_based_permission() in each application schema namespace (remediations.ksl, config_manager.ksl, tasks.ksl).
Remediations Kessel schema (remediations.ksl)
version 0.1
namespace remediations

import rbac


// This is working around a conflict between V1 and V2 permission names.
// As things are right now, we can't follow the naming convention exactly because there will be conflict
// Therefore we are deviating from the ${service}_${resource}_${action} format
@rbac.add_v1_based_permission(app:'remediations', resource:'remediation', verb:'read', v2_perm:'remediations_view_remediation');
@rbac.add_v1_based_permission(app:'remediations', resource:'remediation', verb:'write', v2_perm:'remediations_edit_remediation');
@rbac.add_v1_based_permission(app:'remediations', resource:'remediation', verb:'execute', v2_perm:'remediations_execute_remediation');
@rbac.add_v1_based_permission(app:'remediations', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'remediations_playbook_dispatcher_run_view');


config-manager.ksl
… 
@rbac.add_v1_based_permission(app:'config_manager', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'config_manager_playbook_dispatcher_run_view');
...

tasks.ksl
...
@rbac.add_v1_based_permission(app:'tasks', resource:'playbook_dispatcher_run', verb:'read', v2_perm:'tasks_playbook_dispatcher_run_view');
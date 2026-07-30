package migrations

import surrealgoorm "github.com/kemboi22/surreal-go-orm"

func Migrations() []surrealgoorm.Migration {
	return []surrealgoorm.Migration{
		createTenantsTable{},
		createUsersTable{},
		createTenantMembersTable{},
		createDocumentsTable{},
		createDocumentVersionsTable{},
		createTagsTable{},
		createDocumentTagsTable{},
		createDocumentExtractionsTable{},
		createCommentsTable{},
		createSharesTable{},
		createSigningRequestsTable{},
		createProcessingJobsTable{},
		createAuditEventsTable{},
		createWorkflowRequestsTable{},
		createWorkflowActionsTable{},
	}
}

package migrations

import surrealgoorm "github.com/kemboi22/surreal-go-orm"

func Migrations() []surrealgoorm.Migration {
	return []surrealgoorm.Migration{
		createUsersTable{},
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
		createCategoriesTable{},
		createDocumentCategoriesTable{},
		expandVersioningAndRouting{},
		expandSigningRouting{},
		addFoldersAndDocumentFolderID{},
		addAccessAndForms{},
		makeProcessingJobCompletionOptional{},
		addVisitorRequests{},
		addVisitorOperations{},
		addWorkspaceOperations{},
		addAIJobs{},
		addEmailDelivery{},
	}
}

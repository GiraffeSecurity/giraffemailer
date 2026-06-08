package domain

type CleanupFilter struct {
	AccountID       string `json:"account_id"`
	MailboxName     string `json:"mailbox_name,omitempty"`
	SenderDomain    string `json:"sender_domain,omitempty"`
	SenderEmail     string `json:"sender_email,omitempty"`
	OlderThanDays   int    `json:"older_than_days,omitempty"`
	LargerThanKB    int    `json:"larger_than_kb,omitempty"`
	HasAttachments  *bool  `json:"has_attachments,omitempty"`
	FlagNotSeen     bool   `json:"flag_not_seen,omitempty"`
	SubjectContains string `json:"subject_contains,omitempty"`
}

type CleanupPreview struct {
	Count      int
	TotalBytes int64
}

type CleanupJob struct {
	ID               string
	Name             string
	AccountID        string
	FilterJSON       string
	Action           string
	MoveTargetFolder *string
	CreatedBy        string
	CreatedAt        string
}

type CleanupRun struct {
	ID                string
	Status            string
	TotalCandidates   int
	Processed         int
	SkippedUnarchived int
	FreedBytes        int64
	ErrorMessage      *string
	StartedAt         *string
	FinishedAt        *string
}

type CreateCleanupJobInput struct {
	Name             string
	Filter           CleanupFilter
	Action           string
	MoveTargetFolder *string
	CreatedBy        string
}

type UpdateCleanupJobInput struct {
	Name             string
	Filter           CleanupFilter
	Action           string
	MoveTargetFolder *string
}

type JobExecutionResult struct {
	TotalCandidates   int
	Processed         int
	SkippedUnarchived int
	FreedBytes        int64
}

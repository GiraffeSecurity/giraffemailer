package domain

type AttachmentMeta struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	PartPath    string
}

type MessageSummary struct {
	ID                  string
	UID                 int64
	Subject             *string
	SenderName          *string
	SenderEmail         string
	Date                *string
	SizeBytes           int64
	HasAttachments      bool
	AttachmentCount     int
	BodyPreview         *string
	ArchivedAt          *string
	DeletedFromServerAt *string
	ArchiveState        string
	MailboxName         string
	AccountName         string
}

type MessageDetail struct {
	ID                  string
	Subject             *string
	SenderName          *string
	SenderEmail         *string
	Date                *string
	SizeBytes           int64
	ArchivedAt          *string
	DeletedFromServerAt *string
	ArchiveState        string
	BodyHTML            *string
	BodyText            *string
	Attachments         []AttachmentMeta
}

type MessageListFilter struct {
	AccountID          string
	MailboxID          string
	ScopedAccountIDs   []string
	Sender             string
	ArchiveState   string
	HasAttachments bool
	Unread         bool
	Sort           string
	Desc           bool
	Cursor         string
	Limit          int
}

type MessagePage struct {
	Messages   []MessageSummary
	NextCursor string
	HasMore    bool
	Limit      int
}

type SearchQuery struct {
	Query              string
	Cursor             string
	Limit              int
	ScopedAccountIDs   []string
	Sender             string
	AccountID      string
	HasAttachments bool
	ArchiveState   string
	MinSize        string
}

type SearchResult struct {
	Messages   []MessageSummary
	Total      int
	NextCursor string
	HasMore    bool
	Limit      int
}

type Insights struct {
	TotalMessages    int64
	ArchivedMessages int64
	TotalBytes       int64
	ArchivedBytes    int64
	ReclaimableBytes int64
	TopSenders       []SenderStat
	SizeByYear       []YearStat
}

type SenderStat struct {
	Email string
	Count int
	Bytes int64
}

type YearStat struct {
	Year  string
	Count int
	Bytes int64
}

type Mailbox struct {
	ID                string
	Name              string
	MessageCount      int
	TotalSizeBytes    int64
	ArchivedCount     int
	ArchivedSizeBytes int64
	ArchivedPercent   float64
	LastIndexedAt     *string
	LastArchivedAt    *string
}

type AttachmentDownload struct {
	Filename    string
	ContentType string
	Body        []byte
}

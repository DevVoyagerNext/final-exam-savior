package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Email        string    `gorm:"size:128;not null;uniqueIndex:uk_users_email"`
	PasswordHash string    `gorm:"size:255;not null"`
	Role         string    `gorm:"size:32;not null;index:idx_users_role"`
	Status       string    `gorm:"size:32;not null;index:idx_users_status"`
	RegisteredAt time.Time `gorm:"not null;index:idx_users_registered_at"`
	LastLoginAt  *time.Time
	DisabledAt   *time.Time
	DisabledBy   *uint64
	Remark       *string `gorm:"size:255"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }

type UserSession struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	UserID        uint64    `gorm:"not null;index:idx_user_sessions_user_id"`
	SessionToken  string    `gorm:"size:128;not null;uniqueIndex:uk_user_sessions_token"`
	Status        string    `gorm:"size:32;not null;index:idx_user_sessions_status_expires,priority:1"`
	LoginIP       *string   `gorm:"size:64"`
	UserAgent     *string   `gorm:"size:512"`
	IssuedAt      time.Time `gorm:"not null"`
	ExpiresAt     time.Time `gorm:"not null;index:idx_user_sessions_status_expires,priority:2"`
	InvalidatedAt *time.Time
	InvalidReason *string `gorm:"size:64"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (UserSession) TableName() string { return "user_sessions" }

type InviteCode struct {
	ID             uint64  `gorm:"primaryKey;autoIncrement"`
	Code           string  `gorm:"size:64;not null;uniqueIndex:uk_invite_codes_code"`
	CodeType       string  `gorm:"size:32;not null"`
	BatchNo        *string `gorm:"size:64;index:idx_invite_codes_batch_no"`
	TotalQuota     uint32  `gorm:"not null"`
	RemainingQuota uint32  `gorm:"not null"`
	Status         string  `gorm:"size:32;not null;index:idx_invite_codes_status"`
	Remark         *string `gorm:"size:255"`
	CreatedBy      uint64  `gorm:"not null;index:idx_invite_codes_created_by"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (InviteCode) TableName() string { return "invite_codes" }

type FileCategory struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	Name      string `gorm:"size:64;not null;uniqueIndex:uk_file_categories_name"`
	IsBuiltin bool   `gorm:"not null"`
	Status    string `gorm:"size:32;not null;index:idx_file_categories_status_sort,priority:1"`
	SortNo    int    `gorm:"not null;default:0;index:idx_file_categories_status_sort,priority:2"`
	CreatedBy *uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (FileCategory) TableName() string { return "file_categories" }

type LearningFile struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	SourceFileHash  string    `gorm:"size:128;not null;uniqueIndex:uk_learning_files_hash"`
	SourceFileName  string    `gorm:"size:255;not null;index:idx_learning_files_name"`
	SourceFileType  string    `gorm:"size:128;not null"`
	SourceFileSize  uint64    `gorm:"not null"`
	SourceObjectURL string    `gorm:"column:source_object_url;size:1024;not null"`
	CategoryID      uint64    `gorm:"not null;index:idx_learning_files_category"`
	Visibility      string    `gorm:"size:32;not null;index:idx_learning_files_visibility"`
	UploadUserID    uint64    `gorm:"not null;index:idx_learning_files_upload_user"`
	UploadTime      time.Time `gorm:"not null;index:idx_learning_files_upload_time"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (LearningFile) TableName() string { return "learning_files" }

type FilePreviewRecord struct {
	ID               uint64  `gorm:"primaryKey;autoIncrement"`
	FileID           uint64  `gorm:"not null;uniqueIndex:uk_file_preview_records_file_id"`
	PreviewMode      string  `gorm:"size:32;not null"`
	PreviewStatus    string  `gorm:"size:32;not null;index:idx_file_preview_records_status"`
	PreviewObjectURL *string `gorm:"column:preview_object_url;size:1024"`
	LastSuccessAt    *time.Time
	LastErrorMessage *string `gorm:"size:1024"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (FilePreviewRecord) TableName() string { return "file_preview_records" }

type FileGenerateRecord struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement"`
	FileID          uint64 `gorm:"not null;uniqueIndex:uk_file_generate_records_file_id"`
	TotalStatus     string `gorm:"size:32;not null;index:idx_file_generate_records_total_status"`
	LastGeneratedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (FileGenerateRecord) TableName() string { return "file_generate_records" }

type FileGenerateRecordItem struct {
	ID               uint64  `gorm:"primaryKey;autoIncrement"`
	GenerateRecordID uint64  `gorm:"not null;uniqueIndex:uk_file_generate_record_items_record_type,priority:1"`
	ItemType         string  `gorm:"size:32;not null;uniqueIndex:uk_file_generate_record_items_record_type,priority:2"`
	ItemStatus       string  `gorm:"size:32;not null;index:idx_file_generate_record_items_status"`
	ResultObjectURL  *string `gorm:"column:result_object_url;size:1024"`
	LastSuccessAt    *time.Time
	LastErrorMessage *string `gorm:"size:1024"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (FileGenerateRecordItem) TableName() string { return "file_generate_record_items" }

type GenerateTask struct {
	ID                  uint64  `gorm:"primaryKey;autoIncrement"`
	TaskNo              string  `gorm:"size:64;not null;uniqueIndex:uk_generate_tasks_task_no"`
	FileID              *uint64 `gorm:"index:idx_generate_tasks_file_id"`
	UploadUserID        uint64  `gorm:"not null;index:idx_generate_tasks_upload_user,priority:1"`
	TriggerType         string  `gorm:"size:32;not null"`
	Status              string  `gorm:"size:32;not null;index:idx_generate_tasks_status,priority:1"`
	FileSnapshotName    string  `gorm:"size:255;not null"`
	FileSnapshotHash    string  `gorm:"size:128;not null"`
	FileDeletedSnapshot bool    `gorm:"not null"`
	ReuseExisting       bool    `gorm:"not null;default:false"`
	TaskRemark          *string `gorm:"size:255"`
	StartedAt           *time.Time
	FinishedAt          *time.Time
	LastErrorMessage    *string   `gorm:"size:1024"`
	ExpiresAt           time.Time `gorm:"not null;index:idx_generate_tasks_expires"`
	CreatedAt           time.Time `gorm:"index:idx_generate_tasks_upload_user,priority:2;index:idx_generate_tasks_status,priority:2"`
	UpdatedAt           time.Time
}

func (GenerateTask) TableName() string { return "generate_tasks" }

type GenerateTaskItem struct {
	ID                uint64 `gorm:"primaryKey;autoIncrement"`
	TaskID            uint64 `gorm:"not null;uniqueIndex:uk_generate_task_items_task_type,priority:1"`
	ItemType          string `gorm:"size:32;not null;uniqueIndex:uk_generate_task_items_task_type,priority:2"`
	Status            string `gorm:"size:32;not null;index:idx_generate_task_items_status,priority:1"`
	AutoRetryCount    uint32 `gorm:"not null;default:0"`
	ManualRetryCount  uint32 `gorm:"not null;default:0"`
	MaxAutoRetryCount uint32 `gorm:"not null;default:3"`
	RetryIntervalSec  uint32 `gorm:"column:retry_interval_seconds;not null;default:5"`
	StartedAt         *time.Time
	FinishedAt        *time.Time
	NextRetryAt       *time.Time `gorm:"index:idx_generate_task_items_status,priority:2"`
	LastErrorCode     *string    `gorm:"size:64"`
	LastErrorMessage  *string    `gorm:"size:1024"`
	ResultObjectURL   *string    `gorm:"column:result_object_url;size:1024"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (GenerateTaskItem) TableName() string { return "generate_task_items" }

type PreviewConversionTask struct {
	ID                uint64  `gorm:"primaryKey;autoIncrement"`
	FileID            *uint64 `gorm:"index:idx_preview_conversion_file,priority:1"`
	RequestUserID     uint64  `gorm:"not null"`
	SourceFileType    string  `gorm:"size:128;not null"`
	Status            string  `gorm:"size:32;not null;index:idx_preview_conversion_status,priority:1"`
	AutoRetryCount    uint32  `gorm:"not null;default:0"`
	ManualRetryCount  uint32  `gorm:"not null;default:0"`
	MaxAutoRetryCount uint32  `gorm:"not null;default:3"`
	RetryIntervalSec  uint32  `gorm:"column:retry_interval_seconds;not null;default:5"`
	StartedAt         *time.Time
	FinishedAt        *time.Time
	NextRetryAt       *time.Time `gorm:"index:idx_preview_conversion_status,priority:2"`
	LastErrorMessage  *string    `gorm:"size:1024"`
	PreviewObjectURL  *string    `gorm:"column:preview_object_url;size:1024"`
	ExpiresAt         time.Time  `gorm:"not null;index:idx_preview_conversion_expires"`
	CreatedAt         time.Time  `gorm:"index:idx_preview_conversion_file,priority:2"`
	UpdatedAt         time.Time
}

func (PreviewConversionTask) TableName() string { return "preview_conversion_tasks" }

type TaskRetryLog struct {
	ID            uint64  `gorm:"primaryKey;autoIncrement"`
	BizType       string  `gorm:"size:32;not null;index:idx_retry_logs_biz,priority:1"`
	BizID         uint64  `gorm:"not null;index:idx_retry_logs_biz,priority:2"`
	TaskID        *uint64 `gorm:"index:idx_retry_logs_task,priority:1"`
	RetryMode     string  `gorm:"size:32;not null"`
	RetryNo       uint32  `gorm:"not null"`
	StatusBefore  string  `gorm:"size:32;not null"`
	StatusAfter   string  `gorm:"size:32;not null"`
	TriggerUserID *uint64
	ErrorMessage  *string   `gorm:"size:1024"`
	CreatedAt     time.Time `gorm:"index:idx_retry_logs_biz,priority:3;index:idx_retry_logs_task,priority:2"`
}

func (TaskRetryLog) TableName() string { return "task_retry_logs" }

type SystemNotification struct {
	ID                 uint64  `gorm:"primaryKey;autoIncrement"`
	UserID             uint64  `gorm:"not null;index:idx_notifications_user_status,priority:1;index:idx_notifications_user_type,priority:1"`
	Type               string  `gorm:"size:32;not null;index:idx_notifications_user_type,priority:2"`
	Title              string  `gorm:"size:255;not null"`
	Summary            string  `gorm:"size:512;not null"`
	Content            *string `gorm:"type:text"`
	Status             string  `gorm:"size:32;not null;index:idx_notifications_user_status,priority:2"`
	TargetType         string  `gorm:"size:32;not null"`
	TargetID           *uint64
	TargetSnapshotName *string `gorm:"size:255"`
	ErrorSummary       *string `gorm:"size:1024"`
	MergedKey          *string `gorm:"size:128;index:idx_notifications_merged_key"`
	ReadAt             *time.Time
	ExpiresAt          time.Time `gorm:"not null;index:idx_notifications_expires"`
	CreatedAt          time.Time `gorm:"index:idx_notifications_user_status,priority:3;index:idx_notifications_user_type,priority:3"`
	UpdatedAt          time.Time
}

func (SystemNotification) TableName() string { return "system_notifications" }

func AutoMigrateModels() []any {
	return []any{
		&User{},
		&UserSession{},
		&InviteCode{},
		&FileCategory{},
		&LearningFile{},
		&FilePreviewRecord{},
		&FileGenerateRecord{},
		&FileGenerateRecordItem{},
		&GenerateTask{},
		&GenerateTaskItem{},
		&PreviewConversionTask{},
		&TaskRetryLog{},
		&SystemNotification{},
	}
}

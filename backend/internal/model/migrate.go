package model

func AutoMigrateModels() []any {
	return []any{
		&User{},
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

package model

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

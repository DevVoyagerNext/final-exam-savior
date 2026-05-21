package model

func AutoMigrateModels() []any {
	return []any{
		&User{},
		&InviteCode{},
		&FileCategory{},
		&LearningFile{},
		&FileGenerateRecord{},
		&FileGenerateRecordItem{},
		&GenerateTask{},
		&GenerateTaskItem{},
		&TaskRetryLog{},
		&SystemNotification{},
	}
}

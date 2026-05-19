from pathlib import Path
import re

root = Path(r'C:\Users\15034\GolandProjects\final-exam-savior\backend')
app_raw = (root / 'internal/app/app.go').read_text(encoding='utf-8')
start = app_raw.index('const (')
end = app_raw.index('type RegisterRequest struct {')
body = app_raw[start:end]
body = body.replace('type App struct', 'type Service struct')
body = body.replace('type Service struct {\n\tcfg', 'type Service struct {\n\tdao          *dao.DAO\n\tcfg')

ctor = '''func New(ctx context.Context, cfg config.Config) (*Service, error) {
\tstore, err := dao.New(ctx, cfg)
\tif err != nil {
\t\treturn nil, err
\t}

\tstorage, localStorage, err := buildStorage(cfg)
\tif err != nil {
\t\t_ = store.Close()
\t\treturn nil, err
\t}

\tsvc := &Service{
\t\tdao:          store,
\t\tcfg:          cfg,
\t\tmailer:       platform.NewSMTPMailer(cfg.SMTP),
\t\tcaptcha:      platform.NewGeetestValidator(cfg.Geetest),
\t\tstorage:      storage,
\t\tlocalStorage: localStorage,
\t\tai:           platform.NewOpenAICompatClient(cfg.AI),
\t\tparser:       platform.NewHTTPParser(cfg.Parser),
\t\tconverter:    platform.NewHTTPConverter(cfg.Preview),
\t\thttpClient:   &http.Client{Timeout: 2 * time.Minute},
\t}

\tif cfg.Database.AutoMigrate {
\t\tif err := svc.dao.Gorm().AutoMigrate(model.AutoMigrateModels()...); err != nil {
\t\t\t_ = store.Close()
\t\t\treturn nil, fmt.Errorf("auto migrate: %w", err)
\t\t}
\t}

\tif err := svc.ensureSeeds(ctx); err != nil {
\t\t_ = store.Close()
\t\treturn nil, err
\t}
\tif err := svc.ensureRedisGroups(ctx); err != nil {
\t\t_ = store.Close()
\t\treturn nil, err
\t}
\treturn svc, nil
}
'''
body = re.sub(r'func New\(ctx context\.Context, cfg config\.Config\) \(\*App, error\) \{.*?\n\}', ctor, body, count=1, flags=re.S)
body = re.sub(r'func openDB\(.*?\n\}\n\nfunc buildStorage', 'func buildStorage', body, count=1, flags=re.S)
body = re.sub(r'func \(a \*App\)', 'func (s *Service)', body)
body = re.sub(r'\ba\.', 's.', body)
body = re.sub(r'func \(s \*Service\) Close\(\) error \{.*?\n\}', 'func (s *Service) Close() error {\n\treturn s.dao.Close()\n}\n', body, count=1, flags=re.S)

req_map = {
    'RegisterRequest': 'request.RegisterRequest',
    'LoginRequest': 'request.LoginRequest',
    'ChangePasswordRequest': 'request.ChangePasswordRequest',
    'ResetPasswordRequest': 'request.ResetPasswordRequest',
    'CreateInviteCodeRequest': 'request.CreateInviteCodeRequest',
    'BatchGenerateInviteCodeRequest': 'request.BatchGenerateInviteCodeRequest',
    'ListInviteCodeRequest': 'request.ListInviteCodeRequest',
    'CategoryRequest': 'request.CategoryRequest',
    'UpdateCategoryRequest': 'request.UpdateCategoryRequest',
    'ListFileRequest': 'request.ListFileRequest',
    'ListTaskRequest': 'request.ListTaskRequest',
    'ListNotificationRequest': 'request.ListNotificationRequest',
    'ListUserRequest': 'request.ListUserRequest',
}
for old, new in req_map.items():
    body = body.replace(f'req {old}', f'req {new}')

body = body.replace('s.db', 's.dao.Gorm()')
body = body.replace('s.redis', 's.dao.Redis()')

header = '''package service

import (
\t"context"
\t"crypto/rand"
\t"crypto/sha256"
\t"encoding/hex"
\t"encoding/json"
\t"errors"
\t"fmt"
\t"io"
\t"mime/multipart"
\t"net/http"
\t"sort"
\t"strconv"
\t"strings"
\t"sync"
\t"time"

\t"github.com/golang-jwt/jwt/v5"
\t"github.com/google/uuid"
\t"github.com/redis/go-redis/v9"
\t"golang.org/x/crypto/bcrypt"
\t"gorm.io/gorm"
\t"gorm.io/gorm/clause"

\t"final-exam-savior/backend/internal/config"
\t"final-exam-savior/backend/internal/dao"
\t"final-exam-savior/backend/internal/dto/request"
\t"final-exam-savior/backend/internal/model"
\t"final-exam-savior/backend/internal/platform"
)

'''
(root / 'internal/service/service.go').write_text(header + body, encoding='utf-8')

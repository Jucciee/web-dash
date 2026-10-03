# 제작을 쉽게 하는 Go 패키지 조합

사용자 요청에 따라 웹 서버, DB, 세션, 설정, 시스템 지표, Nginx 문법 분석은 패키지를 사용한다. 직접 작성하는 코드는 후보 추출, 사용자 편집 보존, 가져오기 적용, 카드와 컨테이너 연결 등 앱의 동작 규칙에 집중한다.

## 1. 권장 조합

| 용도 | 패키지 | 덜 작성하게 되는 코드 |
| --- | --- | --- |
| REST API | `github.com/gin-gonic/gin` | route group, JSON binding, validation, middleware |
| DB CRUD | `gorm.io/gorm` | struct mapping, 기본 query, association, transaction |
| SQLite driver | `github.com/ncruces/go-sqlite3/gormlite` | GORM 연결, CGO 없이 개발·빌드 |
| DB migration | `github.com/go-gormigrate/gormigrate/v2` | migration 순서와 적용 이력 |
| 세션 | `github.com/alexedwards/scs/v2` | 세션 token·만료·갱신·로그아웃 |
| 세션 DB 저장 | `github.com/alexedwards/scs/gormstore` | SQLite에 세션을 보관하는 저장소 |
| CSRF | `github.com/gorilla/csrf` | token 발급과 변경 요청 검사 |
| 비밀번호 | `golang.org/x/crypto/bcrypt` | 비밀번호 hash 생성·검사 |
| 환경 설정 | `github.com/kelseyhightower/envconfig` | env를 struct로 변환, default·required 처리 |
| ID | `github.com/google/uuid` | UUID 생성·검증 |
| Nginx 문법 | `github.com/nginxinc/nginx-go-crossplane` | 설정 파일 token·블록·include 구조 분석 |
| Docker API | `github.com/moby/moby/client` | Docker API 응답과 버전 차이 처리 |
| 호스트 지표 | `github.com/shirou/gopsutil/v4` | CPU·RAM·디스크·OS 정보를 읽는 코드 |

Gin의 binding 검증기를 재사용하고 validator를 별도 중복 구성하지 않는다. URL의 http/https 제한, 원본 파일 접근 제한, revision 충돌 같은 도메인 검증은 별도로 작성한다. [Gin 문서](https://gin-gonic.com/en/docs/)

GORM의 공식 SQLite driver는 CGO가 필요하다. 여기서는 C 컴파일러 준비를 줄이기 위해 GORM 저장소에도 안내된 gormlite를 제안한다. gormlite의 기반 드라이버는 CGO 없이 동작하지만 메모리 사용 특성이 있으므로 연결 수를 작게 두고 초기 통합 확인을 한다. [GORM SQLite 선택지](https://github.com/go-gorm/sqlite), [드라이버 설명](https://github.com/ncruces/go-sqlite3)

## 2. GORM 사용 예시

```go
import (
    "github.com/ncruces/go-sqlite3/gormlite"
    "gorm.io/gorm"
)

db, err := gorm.Open(gormlite.Open("file:dashboard.db"), &gorm.Config{})
if err != nil {
    return err
}

// 실제 코드에서는 driver의 connection별 설정 방식으로
// foreign_keys, busy_timeout, WAL을 적용하고 확인한다.
```

사용자에게 반환하는 DTO와 DB model은 구분한다. DB model을 JSON으로 그대로 반환하면 비밀번호나 내부 필드가 섞일 수 있다.

```go
type ServiceModel struct {
    ID          string `gorm:"primaryKey;size:36"`
    GroupID     string `gorm:"not null;index"`
    Name        string `gorm:"not null"`
    URL         string `gorm:"not null"`
    IconKey     string
    Description string
    Hidden      bool
    Position    int
    Revision    int64 `gorm:"not null;default:1"`
}
```

초기 DB migration에서 schema와 외래키·고유 제약을 만든다. model의 index tag만으로 모든 관계 제약이 만들어졌다고 가정하지 않는다.

```go
result := db.WithContext(ctx).
    Model(&ServiceModel{}).
    Where("id = ? AND revision = ?", id, expectedRevision).
    Updates(map[string]any{
        "name": name,
        "hidden": hidden,
        "revision": gorm.Expr("revision + 1"),
    })
```

실제 handler는 result.Error와 RowsAffected를 검사한다. 수정 필드를 map으로 지정하면 false 값을 저장하는 경우도 명확하다. 한 번의 가져오기 적용에는 db.Transaction을 사용한다.

## 3. 세션과 CSRF 연결

SCS가 세션 데이터를 DB에 보관하도록 gormstore를 연결한다. middleware는 Gin router를 표준 http.Handler로 감싸면 된다. 인증된 사용자 ID는 request context의 SCS 세션에서 읽는다. [SCS](https://github.com/alexedwards/scs), [gormstore](https://pkg.go.dev/github.com/alexedwards/scs/gormstore)

연결 순서는 **외부 SCS LoadAndSave → 브라우저용 CSRF middleware → Gin router**로 구성한다. collector 전송 경로는 별도 token handler로 라우팅해 브라우저 세션·CSRF와 분리한다. [gorilla/csrf 문서](https://pkg.go.dev/github.com/gorilla/csrf)

로그인 전 `/auth/csrf`로 CSRF 토큰을 받고 로그인·setup에 사용한다. 로그인 성공 시 SCS RenewToken으로 세션 식별자를 갱신하고 로그아웃 시 Destroy한다. HttpOnly·SameSite cookie를 사용하고 HTTPS 운영에서는 Secure를 켠다. CSRF 인증 키는 비밀 설정으로 영속화한다.

세션 테이블은 gormstore의 형식과 호환되게 초기 migration에 포함한다. startup에서 migration 적용 후 store를 생성한다. 종료 시 store cleanup도 정리한다. gormstore가 정식 태그 대신 commit 버전으로 제공되는 경우 go.mod에 해당 pseudo-version을 고정한다.

bcrypt는 입력을 자르지 않고 최대72바이트를 검사한다. 초기 비밀번호 정책은 최소12자와 최대72 UTF-8바이트를 안내하고 서버에서도 검사한다. 인증 실패 메시지로 계정 존재 여부를 구분하지 않는다.

## 4. Nginx와 호스트 지표

Nginx 문법은 Crossplane에 맡기고 서비스 후보 규칙을 adapter에 둔다. 제공되는 Open·Glob callback을 가상 파일 목록으로 연결하면 패키지 기본 파일 읽기를 대체할 수 있다. [Crossplane](https://github.com/nginxinc/nginx-go-crossplane)

호스트 수집기는 gopsutil의 cpu·mem·disk·host 패키지를 이용한다. 수집기는 실제 호스트에서 실행하고 어떤 OS·환경에서 측정했는지 함께 전송한다. [gopsutil](https://github.com/shirou/gopsutil)

Caddyfile은 공식 Caddy binary의 adapt 기능을 제한된 worker에서 호출한다. 별도 Caddy 문법 파서를 작성하지 않는다. 전체 Caddy 모듈을 API 서버에 직접 포함하는 방식은 실행 환경을 분리하기 어려우므로 현재 구조에서는 선택하지 않는다.

v0.2 구현은 정적 후보 가져오기에 공식 `caddyfile.Tokenize`만 사용한다. 환경·파일을 여는 Parse/adapt와 Caddy 서버 모듈은 실행하지 않는다. 정적 사이트 주소를 읽는 작은 추출기를 연결했으며 전체 Caddy 문법·모듈 검증은 위 worker 확장안으로 남긴다. Nginx는 Crossplane v0.4.89, Caddy lexer는 v2.11.7, Docker SDK는 client v0.6.1을 고정했다.

## 5. 단계별 의존성 도입

첫 CRUD 단계에는 Gin, GORM, gormlite, gormigrate, SCS, gormstore, CSRF, bcrypt, envconfig, UUID만 연결한다. 가져오기 단계에 Crossplane과 Caddy worker를, 상태 조회 단계에 Docker SDK와 gopsutil을 붙인다.

다음은 설치 명령 예시다. 문서 작성 과정에서 실제 설치나 앱 구현을 수행한 것은 아니다. 각 단계에서 호환되는 지원 버전을 정해 go.mod·go.sum에 고정한다.

```powershell
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get github.com/ncruces/go-sqlite3/gormlite
go get github.com/go-gormigrate/gormigrate/v2
go get github.com/alexedwards/scs/v2
go get github.com/alexedwards/scs/gormstore
go get github.com/gorilla/csrf
go get golang.org/x/crypto
go get github.com/kelseyhightower/envconfig
go get github.com/google/uuid
go get github.com/nginxinc/nginx-go-crossplane
go get github.com/moby/moby/client
go get github.com/shirou/gopsutil/v4
```

상태 수집 스케줄은 처음에는 표준 time.Ticker와 context를 사용한다. 외부 retry·HTTP wrapper·전역 DI framework는 실제 반복 코드가 생겼을 때 추가한다. 요청과 연결 lifecycle은 app 패키지에서 명시적으로 연결하면 충분하다.

## 6. 초기에 확인할 조합

Gin JSON binding과 공통 오류, GORM CRUD와 migration, gormlite transaction, gormstore 로그인·재시작·로그아웃을 먼저 확인한다. 이 작은 통합이 완료된 뒤 가져오기 기능을 붙인다. 직접 작성하는 핵심 규칙에는 테스트를 두고, 패키지 자체 기능을 중복 구현하지 않는다.

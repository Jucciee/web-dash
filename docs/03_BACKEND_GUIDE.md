# Go 백엔드 구현 가이드

Go 백엔드는 Gin, GORM, SCS, Crossplane 등 제작을 쉽게 하는 패키지를 우선 사용한다. 코드 블록은 구현 방향 예시이며 실제 프로젝트에서는 의존성 버전에 맞춰 실행 가능한 코드로 연결한다. 전체 조합은 [07_BACKEND_PACKAGES.md](07_BACKEND_PACKAGES.md)를 참고한다.

## 1. 디렉터리 구조

```text
Dashboard/
├─ cmd/
│  ├─ dashboard/main.go
│  └─ host-collector/main.go
├─ internal/
│  ├─ app/                 # 설정, 의존성 연결, 실행과 종료
│  ├─ httpapi/             # Gin router, handler, DTO, middleware
│  ├─ domain/              # 서비스·그룹·후보·상태 타입
│  ├─ services/            # 카드 CRUD, 가져오기 적용, 배치 저장
│  ├─ importer/
│  │  ├─ nginx/            # Crossplane adapter, 후보 추출
│  │  ├─ caddy/            # 격리된 변환 worker 클라이언트
│  │  └─ normalize/        # URL과 후보 식별자 정리
│  ├─ dockerread/          # Docker 조회 전용 SDK wrapper
│  ├─ monitor/             # 수집 스케줄, 캐시, 웹 상태 확인
│  ├─ hostmetrics/         # gopsutil 수집, 지표 수신
│  ├─ storage/             # GORM model, repository, migration
│  ├─ auth/                # SCS, 비밀번호, 초기 설정, CSRF 연결
│  └─ static/              # 빌드된 프론트 제공
├─ web/
├─ deploy/                 # Dockerfile, Compose, worker 설정
├─ testdata/               # Nginx·Caddy 샘플 설정
├─ docs/
├─ go.mod
└─ go.sum
```

HTTP handler는 입력 binding·검증·상태 코드 변환을 담당한다. 카드 저장과 가져오기 규칙은 services에 두고 GORM은 storage, 외부 응답 해석은 adapter에 둔다. 테스트에서 교체할 저장소와 외부 연결에만 interface를 두어 구조를 단순하게 유지한다.

## 2. 구현 시작 순서

1. Go 모듈을 생성하고 Gin과 설정·로그를 연결한다.
2. GORM으로 SQLite 연결을 만들고 gormigrate로 초기 schema를 적용한다.
3. SCS 세션과 관리자 초기 설정·로그인 API를 작성한다.
4. 그룹·카드 CRUD와 배치 저장을 연결한다.
5. 가져오기 preview/commit을 만든 뒤 Crossplane과 Caddy worker를 연결한다.
6. Docker 조회 adapter와 백그라운드 캐시를 붙인다.
7. 웹 상태 확인·gopsutil 호스트 수집기·백업을 구현한다.

배포 주소·DB 경로·worker 주소는 envconfig로 읽는다. 웹에서 바꾸는 연결과 표시 설정은 DB에 저장한다. 비밀번호·세션 토큰·업로드한 설정 전체는 로그에 기록하지 않는다.

## 3. Gin API

Gin의 route group과 ShouldBindJSON을 사용해 공통 HTTP 처리를 줄인다. 서버 timeout과 종료 처리는 표준 http.Server를 함께 사용한다. [Gin 문서](https://gin-gonic.com/en/docs/)

```go
// 구조 예시: 실제 handler는 오류 code와 requestId도 반환한다.
router := gin.New()
router.Use(gin.Recovery())
api := router.Group("/api/v1")
api.Use(requireSession)
api.GET("/services", handlers.ListServices)
api.POST("/services", handlers.CreateService)
api.PATCH("/services/:id", handlers.UpdateService)
api.POST("/imports/preview", handlers.PreviewImport)
api.POST("/imports/:id/commit", handlers.CommitImport)
api.GET("/docker/connections/:id/containers", handlers.ListContainers)
```

로그인·초기 설정은 공개 route group으로, 카드 API는 인증된 group으로 분리한다. collector 전송은 세션 대신 collector token으로 인증한다. 모든 브라우저 변경 요청에는 CSRF를 적용하고 로그인 전에는 공개 CSRF 토큰 조회 API를 사용한다.

Gin의 body binding 전에 요청 본문 크기를 제한한다. middleware가 API 오류를 공통 JSON으로 변환하게 한다. 존재하지 않는 `/api/...` 경로는 JSON 404를 반환하며 SPA fallback으로 보내지 않는다.

`c.Request.Context()`를 service·GORM·외부 adapter까지 전달한다. background goroutine에는 Gin context를 넘기지 않는다. HTTP 서버에 ReadHeaderTimeout, IdleTimeout을 두고 일반 요청·외부 수집 timeout을 분리한다. 종료 시 수집을 취소하고 세션 정리 작업·DB를 닫는다.

## 4. 가져오기 공통 타입

```go
package importer

import "context"

type Bundle struct {
    Entry string
    Files map[string][]byte // 정규화한 가상 경로만 허용
}

type Warning struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    File    string `json:"file,omitempty"`
    Line    int    `json:"line,omitempty"`
}

type Candidate struct {
    ID            string    `json:"id"`
    SourceKey     string    `json:"sourceKey"`
    SuggestedName string    `json:"suggestedName"`
    SuggestedURL  *string   `json:"suggestedUrl"`
    Upstreams     []string  `json:"upstreams"`
    File          string    `json:"file"`
    Line          int       `json:"line"`
    Warnings      []Warning `json:"warnings"`
}

type Result struct {
    Candidates []Candidate `json:"candidates"`
    Warnings   []Warning   `json:"warnings"`
}

type Parser interface {
    Parse(context.Context, Bundle) (Result, error)
}
```

preview에서는 카드에 쓰지 않고 후보와 snapshot을 짧은 유효기간의 import session에 저장한다. commit에서는 소유자·기간·revision·후보ID를 검증하고 사용자 수정값과 원본 후보를 합쳐 적용한다.

카드 생성·출처 연결·commit 상태 갱신은 하나의 GORM transaction에 넣는다. 같은 import를 같은 내용으로 재전송하면 기존 결과를 반환한다. 이미 적용한 import를 다른 내용으로 적용하려 하면409를 반환한다.

## 5. Nginx 파서 adapter

문법 분석은 `github.com/nginxinc/nginx-go-crossplane`을 사용한다. 직접 작성할 부분은 파싱 결과에서 서비스 후보를 찾는 규칙, URL 정리, 기존 카드와 비교하는 로직이다. [Crossplane 저장소](https://github.com/nginxinc/nginx-go-crossplane)

Crossplane ParseOptions의 Open·Glob을 업로드된 가상 파일 묶음으로 연결해 실제 서버 파일을 읽지 않도록 한다. 기본 파일 opener를 그대로 사용하는 것을 피한다. 선택한 버전의 파일 참조 규칙을 확인한다. [파서 옵션](https://github.com/nginxinc/nginx-go-crossplane/blob/main/parse.go)

후보 추출은 다음 순서로 한다.

1. http 안의 server 또는 단독 server 파일에서 명시적 server_name을 모은다.
2. listen 포트와 TLS 지정으로 접속 URL 후보를 만들고, 모호하면 경고를 붙인다.
3. 일반 문자열 location만 공개 경로 후보로 사용한다. 정규식·named location·rewrite 의존은 수동 확인한다.
4. proxy_pass는 내부 upstream으로 기록한다. upstream group이면 해당 server 목록도 확인한다.
5. `_`, 빈 이름, 와일드카드, 변수에서 구체적인 URL을 임의로 만들지 않는다.
6. 동일 접속 주소를 합치고 원본 파일·줄 번호를 유지한다.

include는 업로드된 파일에서만 해석한다. 미제공 파일·순환·과도한 확장을 오류 또는 경고로 변환한다. 원본 절대 경로는 업로드 내 대응 경로를 확인하고 실제 호스트 경로로 열지 않는다.

nginx -T 출력은 file section에서 가상 파일 목록을 복원한다. 전체를 단순 연결하면 include와 중복될 수 있으므로 파일별 관계를 유지한다. 생소한 directive는 분석 가능한 블록을 유지하면서 경고할 수 있도록 파서 옵션을 설정한다.

## 6. Caddy 변환

파일 묶음을 제한된 worker로 보내 공식 caddy adapt를 실행하고 JSON과 경고를 받는다. 고정된 binary와 argument를 사용하며 shell 문자열을 만들지 않는다. Caddy admin API로 설정을 적용하는 코드는 없다.

worker는 요청별 입력만 접근하고 Docker socket·DB·호스트 설정·비밀 환경값을 받지 않는다. 네트워크와 파일 시스템을 OS 수준에서 제한하고 입력·출력량, 시간, 메모리, 동시 실행수를 제한한다. 작업 폴더 변경만으로 파일 접근이 격리되는 것은 아니다.

표준 모듈 binary를 고정하고 커스텀 모듈·환경값 의존은 해결 방법이 있는 오류로 표시한다. JSON route를 순회할 때 상위 host/path 조건을 유지한다. 정적인 사이트 주소와 upstream을 추출하고 경로 wildcard를 접속 URL로 확정하기 전 사용자 확인을 받는다.

## 7. Docker 조회 adapter

```go
package dockerread

import "context"

type Container struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    State  string `json:"state"`
    Health string `json:"health"`
}

type Stats struct {
    CPUPercent  *float64 `json:"cpuPercent"`
    MemoryBytes *uint64  `json:"memoryBytes"`
    LimitBytes  *uint64  `json:"limitBytes"`
}

type Reader interface {
    List(context.Context) ([]Container, error)
    Inspect(context.Context, string) (Container, error)
    Stats(context.Context, string) (Stats, error)
}
```

공식 SDK를 이 interface 안에서 사용하고 연결별 client 생성·재사용·Close를 관리한다. SDK 응답을 그대로 UI에 반환하지 않고 필요한 필드만 DTO로 변환한다.

transport는 사용하는 GET 경로만 허용한다: /_ping, /version, /containers/json, /containers/{id}/json, /containers/{id}/stats. SDK 연결 검사에 필요하면 HEAD /_ping도 허용한다. API version prefix를 검사하고 ID는 조회 목록에서 받은 hex ID로 제한한다. 변경 메서드는 거절한다.

stats body는 반드시 닫고 초기에는 비stream 조회와 공유 캐시를 쓴다. CPU 첫 측정은 null로 처리한다. 컨테이너 CPU의 코어 기준을 화면에 표시하고 RAM cache·limit 기준도 명시한다.

카드 연결은 container ID만 저장하지 않고 같은 connection의 Compose project/service를 우선 사용한다. 없으면 이름 selector를 사용한다. 재생성 후 여러 개가 일치하면 사용자가 다시 선택하도록 한다. healthcheck 없음, 미연결, 수집 실패는 다른 상태로 처리한다.

## 8. GORM 저장과 동시 수정

GORM의 WithContext, Transaction, parameterized query를 사용한다. 수정 필드를 map으로 명시해 false·0 값도 저장되게 하고 요청 전체를 model에 바로 대입하지 않는다. allowed field를 정해 원하지 않는 필드 변경을 막는다.

`WHERE id=? AND revision=?` 조건으로 갱신하고 RowsAffected를 검사한다. revision 충돌은409다. 그룹·카드 정렬은 전체 목록과 revision을 검증해 한 transaction으로 갱신한다. 여러 테이블을 읽어야 하는 초기 계정 생성과 commit에도 DB 제약을 함께 둔다.

gormigrate의 버전별 migration 안에서 schema 변경을 기록한다. 매 실행마다 전체 AutoMigrate에만 의존하지 않는다. 초기 migration에서 AutoMigrate를 사용할 수 있지만 rename·삭제·데이터 이동은 변경 내용을 명시한다.

카드 삭제는 대시보드의 카드와 관계 데이터만 처리한다. source 삭제는 출처 연결을 해제하고 카드는 유지한다. API 변경이 Docker나 프록시 변경으로 이어지는 경로를 만들지 않는다.

## 9. 백업과 검증

백업은 version이 있는 JSON이며 카드·그룹·layout·표시 설정을 담는다. 비밀번호·세션·수집기 token·Docker 비밀정보는 담지 않는다. 복원은 preview 검사 후 transaction으로 적용한다.

검증은 파서의 실제 입력 사례, 업로드 파일 경계, 중복 commit, 사용자 편집 보존, revision 충돌, 세션·초기 계정, Docker 변경 요청 차단을 중심으로 한다. 패키지 내부 동작을 다시 테스트하기보다 앱이 패키지를 연결한 경계를 확인한다. fake HTTP server와 실제 테스트용 Docker 호스트에서 조회만 수행해 마무리한다.

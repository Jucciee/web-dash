# 구현된 기본 골격

2026-10-03 기준 링크 편집, 설정 분리·테마/CSS·가져오기, 사이트·이미지 주소·업로드 아이콘과 주기적인 웹 응답 확인을 구현했다. Docker는 폴더에서 찾은 Compose 파일과 실행 중인 컨테이너를 대조해 실제 공개 포트를 읽는다. Docker 사용량·상태 모니터링, 호스트 수집기, 백업은 후속 단계다.

## 실제 폴더 구조

```text
cmd/dashboard/          Go 서버 실행, timeout, 종료 처리
internal/app/           환경 설정, 비밀 키 파일, 의존성 연결
internal/auth/          관리자 초기 설정, bcrypt, SCS 세션
internal/domain/        API에 노출하는 타입, 입력, 도메인 오류
internal/httpapi/       Gin API, binding, CSRF, 공통 응답, rate limit
internal/services/      카드 입력 검사, 부분 수정 병합
internal/imports/       프록시 후보 추출, Compose 폴더 검색, 조회 전용 Docker SDK
internal/logging/       slog 설정, 콘솔·교체되는 파일 로그
internal/icons/         이미지 검증·업로드 저장, 사이트 아이콘 검색·캐시
internal/monitoring/    HTTP 상태 관측, 4개 worker, 30초 polling·공유 캐시
internal/webfetch/      HTTP/HTTPS·DNS·redirect·timeout을 제한한 읽기 전용 client
internal/storage/       GORM model, migration, 트랜잭션, revision
internal/static/        SPA 배포 파일과 API 404 분리
web/src/app/            Router, QueryClient, 인증 경계, 메뉴
web/src/api/            fetch wrapper, OpenAPI 생성 타입
web/src/components/     버튼, native dialog, 오류·로딩 표시
web/src/features/       인증, 대시보드, 카드 편집, 다음 단계 안내
web/src/styles/         전역 스타일
tokens.css              공통 색상, 폰트, 간격, 상태 토큰
deploy/                 다단계 Dockerfile, 단일 서비스 Compose
testdata/               Nginx·Caddy 가져오기 샘플
scripts/                격리된 브라우저 테스트 서버
```

API handler는 인증·요청 binding을 담당하고, 서비스 계층은 URL·이름 검증과 PATCH 병합을 수행한다. 저장소가 DB 조회, 수정 revision 확인, 여러 테이블의 트랜잭션을 담당한다. DB model과 응답 DTO를 구분한다.

## 저장과 초기 설정

- 첫 실행에 `data/dashboard.db`, `data/csrf.key`, `data/setup-token`을 만든다. 초기 설정 토큰과 CSRF 키는 난수로 생성하고 파일 권한을 제한한다.
- 관리자 생성과 기본 대시보드·그룹 생성을 한 트랜잭션으로 처리한다. `users.singleton`의 고유·CHECK 제약으로 관리자는 한 명만 존재한다.
- 초기 설정 완료 후 생성된 토큰 파일을 삭제한다. DB에 관리자가 존재하면 환경변수 토큰도 사용할 수 없다.
- 환경변수는 `envconfig`, DB 변경 이력은 `gormigrate`를 사용한다. SCS 세션은 같은 SQLite의 `sessions` 테이블에 저장한다.
- SQLite에 WAL, 외래키, 5초 busy timeout, 단일 connection을 적용한다. `_timefmt=sqlite`로 모든 timestamp binding을 UTC로 맞춘다. 이 설정은 로컬 시간대의 `gormstore` 만료 조회에도 필요하다.
- 로그인 시 세션 ID를 갱신하고 로그아웃 시 폐기한다. 세션은 최대 24시간, 미사용 2시간에 만료된다. 로그인·초기 설정 요청은 IP당 분당 10회로 제한한다.

## API와 타입

현재 구현된 API만 [openapi.json](openapi.json)에 기록한다. 전체 후속 계약은 [05_API_SPEC.md](05_API_SPEC.md)에 있다. JSON `data`·`error`·`meta.requestId`, CSRF 헤더, `If-Match` revision을 사용한다.

```powershell
npm --prefix web run api:generate
npm --prefix web run typecheck
```

`web/src/api/schema.ts`는 생성 파일이다. 계약 변경 시 OpenAPI를 먼저 고친 뒤 위 명령으로 재생성한다. 프론트의 DTO 별칭은 생성 타입을 참조한다. 타입 생성 도구의 peer dependency에 맞춰 TypeScript 5.9.3을 사용한다.

## 가져오기와 표시 설정

`settings`와 `imports`를 버전 migration으로 추가한다. settings는 singleton과 revision을 사용하고 false·빈 CSS도 저장한다. 사용자 CSS는 인증된 `/settings/custom.css`에서 제공하며 홈의 link stylesheet로만 로드해 배포 CSP를 유지한다. 설정·관리 페이지에서는 제거한다.

Nginx Crossplane의 Open·Glob은 업로드한 가상 파일 목록에만 연결한다. Caddy는 공식 caddyfile.Tokenize만 호출하고 앱의 후보 추출기가 정적인 주소와 upstream을 읽는다. 서버의 환경값·파일·모듈을 처리하는 Parse/adapt는 호출하지 않는다. 파일 참조 확장에 한계가 있으며 전체 Caddy 유효성 검사와 다르다.

preview는 사용자·대시보드 revision·만료 시각과 정규화한 후보만 저장한다. 원본 파일 내용은 DB에 보관하지 않는다. commit은 URL·그룹 소속·후보 ID·revision·중복 주소를 검사하고 한 트랜잭션으로 카드를 만든다. 같은 Idempotency-Key와 요청 hash의 재전송에는 저장된 결과를 반환한다.

미리보기에서 URL이 비어 있거나 HTTP/HTTPS 접속 주소로 해석할 수 없는 항목은 후보를 만들지 않고 원본 파일·줄·이유를 warnings에 남긴다. 기존에 저장한 링크의 일시적인 연결 실패는 삭제 사유로 처리하지 않는다.

`20261003_link_icons_monitoring` migration은 기존 링크와 관계를 보존하면서 icon mode·URL·asset ID와 웹 상태 확인 사용 여부를 추가한다. 기본 그룹·설명 문구만 링크 표현으로 바꾸고 사용자 편집 문구는 보존한다. 업로드 이미지의 SHA-256을 ID로 쓰며 `data/icons/`에 저장한다. GET 이미지와 upload 모두 인증을 요구하고 업로드에는 CSRF도 검사한다. 파일 이름을 서버 경로로 사용하지 않는다. 원격 HTML·이미지는 각각 256KiB, 아이콘 캐시는 최대 64개이며 같은 이미지 요청을 합친다. 원격 SVG는 제공하지 않는다.

웹 관측은 DB edit revision과 별개로 메모리에 저장한다. 30초 background polling과 화면 조회 시 누락·오래된 항목 확인을 수행하고, 수동 확인은 5초 이내 요청과 진행 중 요청을 합친다. HEAD 미지원(405/501)은 GET으로 확인하며 본문은 읽지 않고 닫는다. 정상·인증 필요·HTTP 오류·접속 실패·timeout·TLS 오류를 구분한다. 클라이언트는 서버의 관측값을 조회하고 외부 사이트에 직접 요청하지 않는다. 로그에는 링크 ID·HTTP 코드·상태·시간만 남긴다.

Docker SDK는 OS 기본 named pipe/socket 또는 지정 endpoint를 사용하며 요청 hook이 HEAD/GET ping과 GET 목록만 허용한다. runtime Ports와 Compose config_files/working_dir 라벨로 대조하고 stopped 컨테이너는 제외한다. YAML에 적힌 published port를 실행 포트로 대체하지 않는다. Docker Desktop의 Windows 경로·WSL 호스트 경로 표기를 정규화한다.

## 다음 확장 지점

1. 가져오기에 원본 snapshot과 신규·변경·동일·사라짐 비교를 붙인다. Nginx -T와 전체 Caddy adapt 검증은 별도 격리 worker에서 확장한다.
2. Docker Inspect·Stats와 주기적 상태 수집을 추가한다. Linux 배포에는 조회 전용 socket proxy와 Compose 폴더 마운트를 구성한다.
3. `internal/hostmetrics/`, `cmd/host-collector/`에 지표 수집을 구현한다. 실제 출처가 없으면 CPU·RAM·디스크 수치를 생성하지 않는다.
4. 드래그 정렬은 dnd kit 단계에서 붙인다. 현재 키보드로 누를 수 있는 순서 이동 버튼과 전체 layout API를 먼저 제공한다.
5. 백업·복원 preview/commit을 별도 migration으로 추가한다.

현재 사용 중인 Crossplane, Caddy lexer, Docker SDK만 연결한다. 호스트 지표 SDK는 수집 단계에서 추가한다.

## 검증 범위

Go 통합 테스트는 실제 SQLite·세션·CSRF 조합으로 초기 설정, 잘못된 URL·인증·CSRF 거절, PATCH의 false·빈 문자열 저장, revision 충돌, 그룹 삭제 조건, 잘못된 배치의 rollback, 카드·세션의 서버 재시작 유지, 요청 제한을 검증한다. 동시 초기 설정과 외래키·WAL·busy timeout도 확인한다.

Playwright는 격리된 `.tmp/e2e-*` DB에서 초기 설정 → 카드 추가·편집·숨김 → 그룹 이동 → 새로고침 → 로그아웃·로그인 → 삭제·되돌리기를 확인한다. 320·375·414·768px 화면과 dialog의 가로 넘침도 검사한다. 삭제 되돌리기는 같은 구성으로 카드를 다시 만들기 때문에 ID는 새로 발급된다.

추가 테스트는 테마·CSS 재시작 유지, 충돌 거절, 가상 파일 경계·include/snippet, 원자적 import rollback·중복·idempotency, 실행 중 Docker 포트·폴더 소속을 확인한다. 브라우저 테스트의 Docker endpoint는 격리된 테스트용 API 서버이며 실제 Docker Desktop 연결·Docker 컨테이너 빌드는 설치된 환경에서 추가 확인해야 한다. 배포 HTML의 CSP 아래에서도 홈 전용 CSS를 확인한다.

Docker CLI가 설치된 환경에서는 추가로 Compose 빌드와 기동을 확인해야 한다.

아이콘 테스트는 상대 경로의 favicon 검색·기본 경로 fallback·원격 캐시·실제 multipart 업로드·잘못된 형식/크기 거절·재시작 유지·이전 DB migration을 확인한다. HTTP 테스트 서버로 정상·redirect·401/403·404/500·HEAD fallback·timeout·TLS 실패와 확인 비활성화를 검증한다. Playwright는 실제 이미지 로딩, 세 가지 아이콘 모드, 정상/인증 필요/확인 꺼짐 상태, 주소 없는 후보 제외와 모바일 편집 창을 확인한다.

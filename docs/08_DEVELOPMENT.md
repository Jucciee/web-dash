# web-dash 구현 구조

2026-10-10 기준 링크·그룹 편집, 시각적 커스터마이징과 미리보기, 배경 업로드, 가져오기, 아이콘·웹 응답 확인, Docker·호스트 지표와 JSON API 애드온을 구현했다. 홈·로그인의 기본 홍보 문구를 제거하고 제품명을 web-dash로 통일했다. 백업·복원과 앱별 애드온 사전 설정 모음은 후속 작업이다.

## 실제 폴더 구조

```text
cmd/dashboard/          Go 서버 실행, timeout, 종료 처리
internal/addons/        선언형 API 요청, 암호화 입력, 응답 경로·표시 변환, 캐시
internal/app/           환경 설정, 비밀 키 파일, 의존성 연결
internal/auth/          관리자 초기 설정, bcrypt, SCS 세션
internal/domain/        API에 노출하는 타입, 입력, 도메인 오류
internal/httpapi/       Gin API, binding, CSRF, 공통 응답, rate limit
internal/services/      카드 입력 검사, 부분 수정 병합
internal/imports/       프록시 후보 추출, Compose 폴더 검색, 조회 전용 Docker SDK
internal/logging/       slog 설정, 콘솔·교체되는 파일 로그
internal/icons/         이미지 검증·업로드 저장, 사이트 아이콘 검색·캐시
internal/monitoring/    페이지 접속 시 HTTP 상태 관측, 4개 worker·공유 캐시
internal/webfetch/      HTTP/HTTPS·DNS·redirect·timeout을 제한한 읽기 전용 client
internal/storage/       GORM model, migration, 트랜잭션, revision
internal/static/        SPA 배포 파일과 API 404 분리
web/src/app/            Router, QueryClient, 인증 경계, 메뉴
web/src/api/            fetch wrapper, OpenAPI 생성 타입
web/src/components/     버튼, native dialog, 오류·로딩 표시
internal/telemetry/     호스트 지표, Docker Stats, 공유 캐시와 수집 제한
internal/backgrounds/   인증된 배경 이미지 검증·저장
web/src/features/       인증, 대시보드, 화면 편집, 가져오기, 상태 화면
web/src/styles/         전역 스타일
tokens.css              공통 색상, 폰트, 간격, 상태 토큰
Dockerfile              다단계 Go·React 컨테이너 빌드
compose.yaml            기본 단일 서비스 배포
compose.monitoring.yaml Linux 호스트 마운트·읽기 전용 Docker proxy overlay
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

웹 관측은 DB edit revision과 별개로 메모리에 저장한다. 홈·링크 관리 페이지 접속과 링크 URL/확인 설정 변경 시 POST `/web-statuses/check`로 한 번 요청한다. 진행 중 요청과 5초 이내 재요청은 합친다. 서버 시작이나 주기적인 timer는 검사를 시작하지 않는다. GET `/web-statuses`는 캐시만 읽고 검사 완료까지 1초 간격으로 조회한 뒤 멈춘다. 오래된 결과 조회와 브라우저 focus/reconnect도 새 검사를 시작하지 않는다. 사용하지 않는 캐시는 조회 시 정리한다. HEAD 미지원(405/501)은 GET으로 확인하며 본문은 읽지 않고 닫는다. 정상·인증 필요·HTTP 오류·접속 실패·timeout·TLS 오류를 구분한다. 클라이언트는 서버의 관측값을 조회하고 외부 사이트에 직접 요청하지 않는다. 로그에는 링크 ID·HTTP 코드·상태·시간만 남긴다.

Docker SDK는 OS 기본 named pipe/socket 또는 지정 endpoint를 사용하며 요청 hook이 HEAD/GET ping과 GET 목록만 허용한다. runtime Ports와 Compose config_files/working_dir 라벨로 대조하고 stopped 컨테이너는 제외한다. YAML에 적힌 published port를 실행 포트로 대체하지 않는다. Docker Desktop의 Windows 경로·WSL 호스트 경로 표기를 정규화한다.

## 다음 확장 지점

1. 가져오기에 원본 snapshot과 신규·변경·동일·사라짐 비교를 붙인다. Nginx -T와 전체 Caddy adapt 검증은 별도 격리 worker에서 확장한다.
2. 앱별 API 애드온 사전 설정과 링크별 컨테이너 연결을 확장한다. 설정 파일 기반 API 애드온, Docker 상태·Stats와 Linux 조회 전용 proxy 예시는 구현했다.
3. 인증된 원격 호스트 수집기와 추세 보관을 확장한다. 현재는 gopsutil의 실제 실행 호스트 또는 Linux 호스트 마운트를 조회한다.
4. 드래그 정렬은 dnd kit 단계에서 붙인다. 현재 키보드로 누를 수 있는 순서 이동 버튼과 전체 layout API를 먼저 제공한다.
5. 백업·복원 preview/commit을 별도 migration으로 추가한다.

현재 Crossplane, Caddy lexer, Docker SDK, gopsutil을 사용한다.

## 커스터마이징과 상태 수집

`20261009_web_dash_customization` migration은 `settings.appearance_json`, `widgets_json`을 추가한다. Appearance·Widgets는 각각 완전한 객체로 갱신하고 `false`·`0`도 그대로 저장한다. 전체 설정 PATCH에는 기존 If-Match와 트랜잭션을 사용한다. 기존 기본 제목·설명만 web-dash와 빈 설명으로 바꾸고 직접 작성한 문구·CSS·링크는 보존한다.

화면 설정은 프리셋·화면·표시 항목·위젯·고급으로 나눈다. `AppearanceSurface`가 저장된 홈과 저장 전 미리보기에 같은 토큰·속성을 적용한다. 1~6열은 모바일에서 줄어들고 그룹 한 줄 배치에서는 카드 열 수로 적용한다. 미리보기에는 일부 실제 링크를 쓰고 상태 수치는 생성하지 않는다. 홈 CSS는 설정·미리보기에서 실행하지 않으므로 홈을 숨긴 CSS도 설정에서 복구할 수 있다.

배경은 PNG/JPEG/WebP, 8MiB·6000px·2400만 화소로 제한한다. SHA-256 ID와 원자적 파일 교체를 사용하고 인증·CSRF·본문 제한을 적용한다. 새 설정이 참조하는 배경 파일의 존재를 검사한다.

`/monitoring/host`, `/monitoring/docker`는 로그인 후 조회한다. 5초 공유 캐시와 singleflight로 여러 탭의 요청을 합친다. 상태 화면을 열어둔 동안만 수동 또는 10/30/60초 간격으로 조회하며 백그라운드 탭의 주기적 조회는 멈춘다. Docker는 전체 목록, info, 실행 중 컨테이너 Stats를 읽고 CPU의 paired sample·Linux 메모리 cache·누적 네트워크를 계산한다. Stats는 최대 4개 동시·개별 3초, 수집 전체는 12초로 제한한다. 구형 Engine 건강 상태는 Status 문자열에서 읽는다. 미지원·실패 지표는 null이며 부분 실패가 전체 목록을 지우지 않는다.

수집기의 요청 hook은 ping/version/info/목록/Stats GET만 허용한다. 가져오기의 기존 EngineReader는 ping/version/목록만 허용한다. 환경변수·로그·Inspect·제어 API는 조회하지 않는다. Linux 호스트 마운트는 proc/sys/root가 모두 설정돼야 host로 표시한다. 기본 컨테이너 실행 환경과 Docker Desktop Linux VM을 Windows 전체 호스트로 표시하지 않는다.

## API 애드온

`20261010_api_addons` migration은 요청·표시 정의, 일반 연결값, 암호화한 보안 값, 활성화·순서·revision을 저장하는 `addons` 테이블을 추가한다. 정의와 설치 값을 분리하며 내보내기는 정의만 읽는다. AES-GCM 키는 `data/addon-secret.key`에 유지하고 앱 종료 시 애드온 수집기의 context와 idle connection을 닫는다.

`internal/addons`가 v1 manifest·참조 키·인증 헤더·JSON Pointer·입력 크기를 검증한다. GET 또는 JSON POST 요청 하나를 지원하고 표현식이나 스크립트를 실행하지 않는다. CSRF를 포함한 dashboard 세션/헤더는 업스트림에 전달하지 않는다. `webfetch`의 실제 IP 검사와 TLS 검증을 사용하고 redirect를 차단한다. 4개 동시·HTTP 6초·수집 전체 8초·JSON 1MiB로 제한한다. 홈 read는 revision별 singleflight와 5초 캐시를 사용한다. 설정·import·서버 시작은 수집을 시작하지 않는다.

보안 입력은 편집 요청에서 키를 생략하면 보존하고 빈 문자열이면 제거한다. 입력 키와 secret 종류가 같은 경우에만 보존하며 origin이 달라지면 재입력을 요구한다. 공개 DTO에는 설정한 secret 키 이름만 제공한다. 응답 preview는 지정 값과 80개·12단계의 선택 목록을 반환하며 입력 secret의 원문·URL/Base64 인코딩을 마스킹한다. secret이 포함된 응답 키/경로는 선택 목록에서 제외한다.

설정 화면은 간편/JSON 편집, 인증 템플릿 버튼, 응답 경로 선택, 테스트 미리보기, 설치·활성화·순서·삭제, 파일 가져오기·내보내기를 제공한다. 연결값은 파일에 넣지 않으며 가져오기만으로 요청하거나 저장하지 않는다. 수정한 표시 규칙은 재테스트 전까지 이전 결과를 새 규칙의 결과처럼 표시하지 않는다. 홈 `.addon-widget`과 `.addon-metric`에는 기존 Appearance 토큰과 홈 전용 CSS가 적용된다.

작성 규약과 예제는 [09_ADDONS.md](09_ADDONS.md)에 있다. Go 테스트는 secret 암호화·재시작·생략 시 보존·origin 변경·revision 충돌·캐시/동시 요청·redirect/헤더 주입·응답 크기·JSON Pointer·0/false·비활성화와 인증/CSRF를 검증한다. Playwright는 작성→연결 테스트→값 선택→홈 표시→내보내기→공유 파일 가져오기→설치→편집·순서·삭제와 모바일 레이아웃을 확인한다.

## 검증 범위

홈은 `settings.homeLayout`의 섹션과 링크·애드온·호스트·Docker 참조를 함께 배치할 수 있다. `/settings/layout`은 항목과 섹션의 포인터/터치 이동·크기 조절, 키보드·숫자 좌표 편집, 섹션 이름·너비·최소 높이·접기, 되돌리기·취소·저장·충돌 처리를 제공한다. 홈과 섹션 내부는 각각 12열이며 충돌은 아래로 밀어 해결한다. 이전 배치의 생략된 섹션 좌표는 편집 화면에서 계산한 뒤 다음 저장에 포함한다. 고정 높이 링크 카드는 이름을 우선 확보하고 설명·주소·상태를 남는 공간에 표시한다. 기존 자동 배치와 링크 관리 그룹은 보존한다. 저장·검증 규약과 사용 방법은 [10_HOME_LAYOUT.md](10_HOME_LAYOUT.md)에 있다.

Go 통합 테스트는 실제 SQLite·세션·CSRF 조합으로 초기 설정, 잘못된 URL·인증·CSRF 거절, PATCH의 false·빈 문자열 저장, revision 충돌, 그룹 삭제 조건, 잘못된 배치의 rollback, 카드·세션의 서버 재시작 유지, 요청 제한을 검증한다. 동시 초기 설정과 외래키·WAL·busy timeout도 확인한다.

Playwright는 격리된 `.tmp/e2e-*` DB에서 초기 설정 → 카드 추가·편집·숨김 → 그룹 이동 → 새로고침 → 로그아웃·로그인 → 삭제·되돌리기를 확인한다. 320·375·414·768px 화면과 dialog의 가로 넘침도 검사한다. 삭제 되돌리기는 같은 구성으로 카드를 다시 만들기 때문에 ID는 새로 발급된다.

추가 테스트는 테마·CSS 재시작 유지, 충돌 거절, 가상 파일 경계·include/snippet, 원자적 import rollback·중복·idempotency, 실행 중 Docker 포트·폴더 소속을 확인한다. 브라우저 테스트의 Docker endpoint는 격리된 테스트용 API 서버이며 실제 Docker Desktop 연결·Docker 컨테이너 빌드는 설치된 환경에서 추가 확인해야 한다. 배포 HTML의 CSP 아래에서도 홈 전용 CSS를 확인한다.

Docker CLI가 설치된 환경에서는 추가로 Compose 빌드와 기동을 확인해야 한다.

아이콘 테스트는 상대 경로의 favicon 검색·기본 경로 fallback·원격 캐시·실제 multipart 업로드·잘못된 형식/크기 거절·재시작 유지·이전 DB migration을 확인한다. HTTP 테스트 서버로 정상·redirect·401/403·404/500·HEAD fallback·timeout·TLS 실패와 확인 비활성화를 검증한다. Playwright는 실제 이미지 로딩, 세 가지 아이콘 모드, 정상/인증 필요/확인 꺼짐 상태, 주소 없는 후보 제외와 모바일 편집 창을 확인한다.

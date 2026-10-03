# Dashboard 개발 문서

작성일: 2026-10-03

Nginx 설정 또는 Caddyfile에서 서비스를 가져와 대시보드에 추가하고, Docker와 서버 상태를 읽기 전용으로 확인하는 개인 홈서버 대시보드를 만든다. 카드 추가, 가져오기, 배치, 연결 설정, 백업은 브라우저에서 처리한다.

백엔드는 **Go + Gin + GORM**, 프론트엔드는 **React + TypeScript + Vite**, 저장소는 **SQLite**를 사용한다. 관리자 설정·로그인, 링크·그룹 편집, 테마·사용자 CSS, 아이콘, 웹 상태 확인, Docker 폴더 검색과 Nginx·Caddy 설정 가져오기를 구현했다. 홈에는 검색과 링크 카드를 표시하고 편집 기능은 별도 설정 페이지에 둔다. Docker·호스트 지표와 백업은 후속 단계다.

## 빠른 실행

Go **1.26 이상**과 Node.js **24 LTS**가 필요하다. Windows에서도 C 컴파일러 없이 SQLite를 사용할 수 있다. 현재 검증 환경은 Go 1.26.4, Node.js 24.21.0이다. 주요 의존성은 Gin 1.12.0, GORM 1.31.2, gormlite 0.34.0, React 19.3.0, TypeScript 5.9.3, Vite 8.3.2이며 정확한 버전은 lock 파일에 고정한다. Vite의 Node 요구사항은 [공식 안내](https://vite.dev/guide/)를 참고한다.

프로젝트 루트에서 실행한다.

```powershell
# 첫 터미널: API 서버
go mod download
go run ./cmd/dashboard
```

```powershell
# 두 번째 터미널: React 개발 서버
npm --prefix web ci
npm --prefix web run dev
```

[http://127.0.0.1:5173](http://127.0.0.1:5173)에 접속한다. 처음에는 관리자 설정 화면이 열린다. 첫 터미널에 안내된 `data/setup-token` 파일의 값을 입력하고 관리자 아이디·비밀번호를 만든다.

```powershell
Get-Content ./data/setup-token
```

비밀번호는 12자 이상, UTF-8 기준 72바이트 이하로 입력한다. 설정이 완료되면 초기 토큰 파일은 삭제된다. 카드와 세션, CSRF 키는 `data/`에 유지되며 이 폴더는 Git에서 제외된다.

Vite의 `/api` 프록시가 Go 서버로 요청을 전달한다. 브라우저 주소의 호스트를 `127.0.0.1`로 통일하면 세션·CSRF 쿠키를 같은 origin에서 사용할 수 있다.

## 현재 사용할 수 있는 기능

- 초기 관리자 계정 생성, 로그인·로그아웃, 재시작 후 세션 유지
- 홈서버 앱·일반 웹사이트 링크 추가·수정·숨김·삭제, 삭제 되돌리기, 이름·주소·설명 검색
- 사이트 아이콘 자동 가져오기, 아이콘 이미지 주소, PNG/JPG/WebP/ICO 파일 업로드
- 서버의 주기적인 HTTP 상태 확인과 수동 확인, 링크별 확인 사용 여부
- 그룹 추가·이름 변경·빈 그룹 삭제, 카드의 그룹 변경
- 키보드로 조작 가능한 카드 순서 이동, revision을 검사하는 배치 저장 API
- SQLite migration, 실제 외래키, WAL, 트랜잭션, 수정 충돌·CSRF·로그인 요청 제한
- 별도 설정 페이지, 밝게·어둡게·시스템 테마, 강조색·카드 밀도, 홈 전용 사용자 CSS
- Docker Compose 폴더 검색과 실행 중 컨테이너의 실제 공개 TCP 포트 가져오기
- Nginx·Caddy 파일 업로드·붙여넣기, 후보 미리보기·선택 등록·중복 주소 검사
- Go 요청 로그와 시작·종료 로그, 파일 보관과 로그 레벨·형식 설정
- 반응형 화면, OpenAPI 계약과 생성 DTO, Go 통합 테스트·Playwright 브라우저 테스트

Docker 사용량·상태의 주기적 모니터링, 호스트 지표, 백업·복원은 아직 구현하지 않았다. Docker 폴더 검색은 가져오기 시점의 실행 상태와 공개 포트만 조회한다.

## 홈과 설정

- `/`: 링크 검색과 그룹별 바로가기, 웹 상태, 현재 시각. 숨긴 카드와 편집 버튼은 표시하지 않는다.
- `/settings`: 홈 제목·설명, 테마, 강조색, 카드 밀도, 사용자 CSS.
- `/settings/services`: 카드·그룹 편집, 숨김, 순서 변경, 삭제·되돌리기.
- `/settings/imports`: Docker 폴더 검색과 Nginx·Caddy 가져오기.

사용자 CSS는 저장 후 홈에만 적용된다. `.home-dashboard`, `.service-card`, `.service-title`, `.service-description`, `.group-title`을 사용할 수 있다. 최대 32KiB이고, 설정 페이지에서는 CSS가 적용되지 않아 항상 수정할 수 있다. 외부 이미지·폰트는 기본 CSP 정책에 따라 차단된다.

## 링크 아이콘과 웹 상태

링크 추가·편집에서 **사이트 아이콘 자동**, **이미지 주소**, **파일 업로드**, **기본 아이콘**을 선택한다. 사이트 아이콘은 HTML의 icon/apple-touch-icon 링크와 `/favicon.ico`를 순서대로 찾고, 없으면 기본 아이콘을 표시한다. 원격 아이콘은 서버에서 가져와 같은 origin으로 제공한다. PNG, JPG, WebP, ICO를 지원하며 SVG는 지원하지 않는다. 업로드는 최대 1MiB·2048px, 원격 이미지와 HTML은 최대 256KiB이며 업로드 파일은 `data/icons/`에 보존한다.

웹 상태는 **Dashboard 서버에서** 30초마다 HTTP HEAD로 확인한다. HEAD가 405·501을 반환하면 GET으로 다시 확인한다. 정상(2xx/3xx), 인증 필요(401/403), HTTP 오류, 연결 실패, 시간 초과를 구분한다. 카드의 상태 설명에서 HTTP 코드·응답 시간·확인 시각을 볼 수 있고 **지금 확인**으로 즉시 확인을 요청할 수 있다. 링크별 5초 이내의 재요청은 합친다. 즐겨찾기만 필요한 링크는 편집 창의 **웹 상태 확인**을 끈다.

확인은 쿠키나 로그인 정보를 전달하지 않고, 최대 4개 동시 요청·6초 제한·5번 미만 redirect로 수행한다. 사설 LAN과 loopback 앱을 확인할 수 있으며, DNS에서 얻은 실제 IP와 redirect 목적지를 검사하고 link-local·multicast·미지정 주소는 제외한다. TLS 인증서 검증 실패도 표시한다. 응답이 정상이어도 실제 로그인 후 기능이나 Docker 실행 상태를 검증한 결과는 아니다. 관측값은 메모리에 보관해 재시작 후 다시 확인한다.

## 설정 가져오기

Docker는 서버에서 접근 가능한 절대 폴더 경로를 입력한다. Windows에서 Go 서버를 직접 실행한다면 `D:\Docker`처럼 입력하고 Docker Desktop을 실행해둔다. `compose.yml`, `compose.yaml`, `docker-compose.yml`, `docker-compose.yaml`을 하위 4단계까지 찾는다. 숨김 폴더·node_modules·심볼릭 링크는 건너뛴다.

파일의 서비스 이름과 실행 중인 컨테이너의 Compose 경로 라벨을 대조하고 **Docker가 실제로 공개한 포트**를 가져온다. 중지한 컨테이너·미공개 포트는 카드로 만들지 않는다. TCP 포트가 웹 서비스라는 보장은 없으므로 미리보기에서 HTTP/HTTPS 주소를 확인한다. `homepage.href` 라벨이 있으면 그 주소를 우선 사용한다. 접속 호스트에는 브라우저에서 접근할 Docker 서버 IP나 이름을 입력한다.

Windows 기본 연결은 Docker Engine named pipe, Linux 기본 연결은 `/var/run/docker.sock`이다. 다른 데몬이나 조회 전용 socket proxy는 `DASHBOARD_DOCKER_HOST` 또는 `DOCKER_HOST`로 지정한다. 클라이언트는 ping과 컨테이너 목록 조회만 허용한다. Docker 컨텍스트 설정은 자동으로 읽지 않는다.

Nginx와 Caddy는 설정 파일을 업로드하거나 내용을 붙여넣는다. 여러 파일이면 include/import 파일도 함께 올리고 파일 경로와 분석 시작 파일을 선택한다. 경로는 가상 상대 경로이며 서버의 파일로 해석하지 않는다. Nginx는 정적인 server_name·listen·location·proxy_pass, Caddy는 정적인 사이트 주소와 reverse_proxy·단순 snippet·최상위 파일 import를 추출한다. 변수·와일드카드·정규식 등으로 접속 주소를 찾지 못한 항목은 후보에서 제외하고 이유를 표시한다. Caddy 전체 adapt와 모듈 검증, 내부 import 파일 확장, nginx -T 출력은 아직 지원하지 않는다.

미리보기에서 링크 이름·접속 주소·그룹을 수정한 뒤 선택 등록한다. 최대 50개 파일, 개별 2MiB·전체 10MiB, 후보 500개이며 미리보기는 30분간 유효하다. 이미 등록한 주소는 중복으로 표시하고 선택에서 제외한다. 선택한 카드는 한 트랜잭션으로 저장되며 재전송으로 두 번 등록되지 않는다. 기존 카드의 이름이나 그룹은 가져오기로 덮어쓰지 않는다.

## Go 로그

기본 로그는 콘솔과 `data/logs/dashboard.log`에 기록된다. 요청 ID·메서드·경로·응답 코드·처리 시간·응답 크기를 남기며, 비밀번호·쿠키·요청 본문·쿼리 값은 기록하지 않는다. 파일은 10MiB에서 교체하고 이전 파일 최대 3개·7일을 보관한다.

```powershell
Get-Content .\data\logs\dashboard.log -Tail 30 -Wait
# 다음 실행부터 JSON 구조화 로그 사용
$env:DASHBOARD_LOG_FORMAT='json'
$env:DASHBOARD_LOG_LEVEL='info'
```

레벨은 debug·info·warn·error, 형식은 text·json이다. `DASHBOARD_LOG_FILE`로 파일 경로를 지정하고 `-`를 지정하면 콘솔에만 출력한다.

## 빌드와 검증

```powershell
go test ./...
go vet ./...
npm --prefix web run api:generate
npm --prefix web run build
```

React를 빌드한 뒤 Go 서버만 실행하면 [http://127.0.0.1:8080](http://127.0.0.1:8080)에서 API와 SPA를 함께 제공한다.

```powershell
go run ./cmd/dashboard
```

브라우저 테스트는 최초 한 번 Chromium을 설치한 뒤 실행한다. 테스트마다 별도 DB를 사용하고 실제 `data/`는 건드리지 않는다.

```powershell
cd web
npx playwright install chromium
npm run test:e2e
```

환경변수 목록은 [.env.example](.env.example)에 있다. Go는 `.env`를 자동으로 읽지 않으므로 PowerShell의 `$env:DASHBOARD_ADDR='127.0.0.1:8080'`처럼 설정한다. HTTPS로 운영할 때는 `DASHBOARD_COOKIE_SECURE=true`를 적용한다.

## Docker 실행

```powershell
docker compose -f deploy/compose.yaml up -d --build
```

기본 공개 주소는 [http://127.0.0.1:8080](http://127.0.0.1:8080)이고 DB는 `dashboard-data` 볼륨에 저장된다. Docker 이미지에 shell이 없으므로 초기 토큰 파일만 복사해 다음 명령으로 확인한다.

```powershell
docker compose -f deploy/compose.yaml cp dashboard:/data/setup-token ./setup-token
Get-Content ./setup-token
Remove-Item -LiteralPath ./setup-token
```

LAN 공개가 필요하면 Compose 실행 전에 `$env:DASHBOARD_BIND='0.0.0.0'`를 설정한다. 이 단계의 Compose에는 Docker socket이나 프록시 연동을 추가하지 않았다. Docker 빌드·기동은 Docker가 설치된 환경에서 확인해야 한다.

컨테이너에서 Docker 폴더 검색을 사용하려면 Compose 폴더를 컨테이너에 읽기 전용으로 마운트하고 조회 전용 Docker socket proxy를 연결한다. Compose 경로 라벨과 대조할 수 있도록 호스트와 동일한 절대 경로로 폴더를 마운트한다. 기본 배포 파일은 socket을 자동으로 연결하지 않는다.

## 읽는 순서

| 문서 | 내용 |
| --- | --- |
| [01_PROJECT_PLAN.md](docs/01_PROJECT_PLAN.md) | 제품 목표, 범위, 사용자 흐름, 일정과 완료 기준 |
| [02_ARCHITECTURE.md](docs/02_ARCHITECTURE.md) | 시스템 구조, 기술 선택, 데이터 모델, 배포 방식 |
| [03_BACKEND_GUIDE.md](docs/03_BACKEND_GUIDE.md) | Go 패키지 구조, 가져오기 파서, Docker 조회, 구현 예시 |
| [04_FRONTEND_GUIDE.md](docs/04_FRONTEND_GUIDE.md) | 화면 구성, 컴포넌트, 상태 관리, UI 구현 예시 |
| [05_API_SPEC.md](docs/05_API_SPEC.md) | REST API, 요청·응답 형식, 오류와 동시 수정 규칙 |
| [06_TASKS.md](docs/06_TASKS.md) | 구현 순서, 작업 목록, 검증 기준 |
| [07_BACKEND_PACKAGES.md](docs/07_BACKEND_PACKAGES.md) | 제작을 쉽게 하는 Go 패키지 조합, 설치와 사용 기준 |
| [08_DEVELOPMENT.md](docs/08_DEVELOPMENT.md) | 실제 구현 구조, 현재 범위, 검증과 다음 확장 지점 |
| [openapi.json](docs/openapi.json) | 현재 구현한 API의 OpenAPI 3.1 계약 |

## 확정된 요구

- Nginx 설정과 Caddyfile을 가져와 링크 카드로 등록한다.
- Docker는 컨테이너 상태와 사용량을 조회하는 읽기 전용 연동이다.
- Docker·Nginx·Caddy 제어 기능은 현재 개발 범위에 포함하지 않는다.
- 대시보드의 일상적인 설정과 편집은 웹에서 가능해야 한다.
- 호스트 CPU·RAM·디스크 사용량과 컨테이너 사용량을 구분한다.
- 설치 뒤 처음 사용하는 사람도 설정 파일 문법을 몰라도 쓸 수 있어야 한다.

## 설계 가정

첫 설치 대상은 Linux 홈서버이며, Docker 호스트 한 대, 관리자 계정 하나, 기본 대시보드 하나로 시작한다. 개발은 Windows에서도 가능하게 구성한다. Docker Desktop에서 수집하는 Linux VM 지표를 Windows 전체 지표로 표시하지 않는다.

설정 파일 업로드·붙여넣기는 기본 기능이다. 마운트한 설정 폴더에서 가져오기와 주기적인 변경 감지는 후속 작업으로 둔다. 서버의 Docker 접근 권한과 호스트 수집기 설치는 초기 배포 때 설정하고, 그 뒤의 사용은 웹 UI를 중심으로 한다.

## 첫 개발 목표

카드 저장·설정 분리·가져오기 미리보기와 선택 등록까지 구현했다. 다음에는 재가져오기 변경 비교, Docker 상태·사용량 모니터링, 호스트 지표와 백업·복원을 연결한다.

구체적인 의존성 버전은 `go.mod`, `go.sum`, `web/package-lock.json`과 컨테이너 이미지 태그에 고정했다. 남은 기능은 [개발 작업 목록](docs/06_TASKS.md)에 정리했다.

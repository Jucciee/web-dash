# API 애드온 만들기와 공유

web-dash 애드온은 **요청과 표시 규칙을 담은 JSON 설정 파일**이다. 자신이 사용하는 API를 직접 연결하거나 다른 사람이 만든 파일을 가져올 수 있다. 파일에는 API 주소 템플릿·입력 항목·인증 헤더·응답 경로·표시 형식을 넣는다. 서버 주소·토큰·쿠키 등 설치마다 달라지는 실제 연결값은 web-dash에서 따로 입력한다.

## 화면에서 만들기

1. **설정 → 애드온 → 새 애드온**을 연다.
2. 이름과 API 주소를 입력한다. **서버 주소를 입력 항목으로 분리**를 누르면 주소의 서버 부분을 `{{baseUrl}}`로 바꿔 공유하기 편한 설정을 만든다.
3. API가 요구하는 **Bearer 토큰·API 키·쿠키·Basic 인증**을 선택하고 **내 연결값**에 실제 값을 입력한다. 임의 헤더나 입력 항목도 추가할 수 있다.
4. **연결 테스트**를 눌러 JSON 응답을 확인한다. **응답에서 값 선택**의 경로를 클릭하면 표시 항목에 추가된다.
5. 표시 이름·형식·단위·배수·소수 자릿수를 정한다. 규칙을 바꾼 뒤 다시 테스트하면 표시 결과를 확인할 수 있다.
6. 지표 카드 또는 목록, 갱신 간격을 고르고 **애드온 설치**를 누른다. 홈에 위젯으로 표시된다.

설치한 애드온은 설정에서 켜기·끄기, 편집, 위아래 이동, 삭제할 수 있다. **JSON 설정**에서는 같은 정의를 직접 편집한다. 수정한 JSON을 적용하거나 간편 편집으로 전환하면 형식을 검사한다. 잘못된 JSON은 적용되지 않는다.

## 다른 사람의 파일 가져오기

**JSON 가져오기**로 `.web-dash.json` 또는 `.json` 파일을 고른다. 편집 화면에서 API 요청과 표시 규칙을 확인한 뒤 자신의 서버 주소·토큰·쿠키를 입력하고 연결 테스트한다. 가져오기 자체는 API 요청이나 저장을 수행하지 않는다. 연결값을 나중에 넣으려면 **홈에서 사용**을 끄고 설치한다.

**JSON 내보내기**는 애드온 정의만 저장한다. 내 연결값과 서버에 저장한 인증값은 파일에 포함되지 않는다. 개인 서버 주소까지 파일에서 제외하려면 입력 항목으로 분리하고 `default`를 빈 값으로 둔다. `request.url`·헤더·본문·일반 입력의 `default`에 직접 쓴 값은 정의의 일부로 공유된다.

시작용 예제:

- [Bearer 인증 JSON API](../examples/addons/json-api.web-dash.json)
- [Cookie 인증 JSON API와 목록](../examples/addons/cookie-api.web-dash.json)
- [POST·GraphQL과 API 키](../examples/addons/graphql.web-dash.json)

예제의 `/api/status`·GraphQL 쿼리·응답 경로는 형식을 보여주는 샘플이다. 사용하려는 앱의 API 경로와 JSON 구조에 맞게 수정한다.

## 설정 파일 구조

| 항목 | 역할 |
| --- | --- |
| `schemaVersion` | 현재 `1` |
| `name`, `icon` | 위젯 이름과 아이콘 |
| `display` | `stats` 지표 카드, `list` 목록 |
| `refreshSeconds` | `0` 수동, `10`·`30`·`60`·`300`초 |
| `request` | `method`, `url`, `headers`, `body` |
| `inputs` | 사용자가 입력할 서버 주소·인증값 등의 정의 |
| `fields` | 응답에서 가져올 값과 표시 규칙 |

아이콘은 `server`, `photos`, `media`, `files`, `code`, `home`, `network`, `database` 중 선택한다. v1은 애드온당 HTTP 요청 하나의 JSON 응답을 처리한다. 요청 방식은 GET 또는 조회용 POST(JSON)이며 일반 REST API·GraphQL을 연결할 수 있다. 여러 단계의 로그인·토큰 자동 갱신·HTML 파싱·임의 JavaScript 실행은 지원 범위에 포함되지 않는다.

### 입력 항목과 인증

```json
{"key":"token","label":"API 토큰","type":"secret","required":true,"default":""}
```

`key`는 영문자로 시작하는 영문·숫자·밑줄 40자 이내다. `type: "text"`는 일반 값이고 `secret`은 보안 값이다. `required`는 필수 여부, `default`는 일반 입력의 기본값이다. 보안 입력의 기본값은 비워 둔다.

URL·헤더·POST 본문에서 입력값을 다음처럼 참조한다.

| 템플릿 | 동작 | 예 |
| --- | --- | --- |
| `{{key}}` | 그대로 대입 | `{{baseUrl}}/api/status` |
| `{{key\|url}}` | 쿼리 파라미터용 URL 인코딩 | `/api/items?filter={{filter\|url}}` |
| `{{key\|json}}` | 따옴표와 줄바꿈을 처리한 JSON 문자열 | `{"query":{{query\|json}}}` |
| `{{key\|base64}}` | Base64 인코딩 | `Basic {{credentials\|base64}}` |

`json` 템플릿의 바깥에 따옴표를 추가하지 않는다. Basic 인증 입력값은 `아이디:비밀번호` 형태다. 쿠키는 API가 요구하는 `session=값; other=값` 문자열을 직접 입력한다. web-dash는 브라우저의 다른 사이트 쿠키를 읽거나 로그인 쿠키를 자동 수집하지 않는다.

인증 헤더 예:

```json
[
  {"name":"Authorization","value":"Bearer {{token}}"},
  {"name":"Cookie","value":"{{cookie}}"}
]
```

`Authorization`, `Cookie`, `X-API-Key`, `Api-Key`, `X-Auth-Token`은 `secret` 입력을 참조해야 한다. 다른 이름의 인증 헤더도 입력을 `secret`으로 정의해 참조한다. URL 쿼리나 POST 본문에 필요한 토큰도 같은 방식으로 쓸 수 있다.

저장한 보안 값은 편집 화면에서 빈 입력으로 보인다. 값을 건드리지 않으면 기존 값을 유지하고 새 값을 입력하면 교체한다. 입력했다가 지우면 해당 값은 제거된다. API의 origin(프로토콜·호스트·포트)을 바꾸면 인증값을 다시 입력해야 한다.

### 응답 경로와 표시

```json
{
  "active": 0,
  "load": 0.25,
  "memoryBytes": 209715200,
  "uptimeSeconds": 90000,
  "status": "healthy"
}
```

첫 번째 예제에 이 응답을 넣으면 `0개`, `25.0%`, `200.0 MiB`, `1일 1시간`, `정상`으로 표시한다.

`path`는 JSON Pointer다. `/data/count`는 중첩 값, `/items/0/name`은 배열의 첫 항목, 빈 경로는 응답 전체다. 키에 `/`가 들어 있으면 `~1`, `~`는 `~0`으로 쓴다. 예: `{"a/b":{"~key":0}}`의 경로는 `/a~1b/~0key`다.

| `format` | 표시 |
| --- | --- |
| `text` | 문자열·숫자·불리언 |
| `number` | 숫자에 배수와 소수 자릿수 적용 |
| `percent` | 계산한 값에 `%` 붙이기 |
| `bytes` | 바이트를 B·KiB·MiB·GiB 등으로 변환 |
| `duration` | 초를 일·시간·분·초로 표시 |
| `boolean` | 예 / 아니요 |
| `count` | 배열 길이 또는 객체 키 개수 |

숫자 변환의 기본 배수는 `1`이다. `0.25`를 `25.0%`로 표시하려면 `format: "percent"`, `scale: 100`, `decimals: 1`을 쓴다. 이미 0~100 단위인 값은 배수를 `1`로 둔다. `unit`은 값 뒤의 단위이며 퍼센트·용량의 자동 단위를 중복해서 넣을 필요는 없다. `valueMap`은 원래 값별 표시 문구를 정한다. 예: `{"healthy":"정상","degraded":"점검 필요"}`. 이 매핑은 JSON 편집에서 설정한다.

`0`과 `false`도 정상 값으로 표시한다. 경로가 없거나 형식이 맞지 않으면 `—`와 확인 안내를 표시한다. 연결 테스트의 선택 목록은 최대 80개·12단계까지 보여 주며 더 깊은 값은 직접 경로를 입력할 수 있다.

## 연결과 저장

API는 web-dash 서버가 호출하므로 주소는 **서버에서 접근 가능한 주소**여야 한다. Docker에서 실행할 때 `localhost`는 web-dash 컨테이너다. 같은 Compose 네트워크의 서비스 이름이나 접근 가능한 LAN 주소를 사용한다. 서버 호출이므로 외부 API의 브라우저 CORS 설정에 영향을 받지 않는다.

인증값은 SQLite의 애드온 레코드에 AES-GCM으로 암호화해 저장한다. 암호화 키는 `data/addon-secret.key`에 유지한다. 재시작·이전·백업 시 **SQLite와 이 키를 함께 보존**한다. API와 내보내기는 저장된 인증값을 반환하지 않는다. 테스트 응답에서 알려진 입력 보안 값은 마스킹한다.

리다이렉트는 따라가지 않으며, 3xx가 나오면 최종 JSON API 주소를 입력한다. TLS 인증서 검증을 수행한다. LAN·loopback은 사용할 수 있고 link-local·multicast·미지정 IP는 제한한다. 쿠키·토큰은 설정한 요청에만 넣으며 응답의 `Set-Cookie`를 저장하지 않는다.

홈에 활성화된 위젯이 있을 때만 자동 갱신한다. 백그라운드 탭의 주기적 갱신은 멈추고, 여러 탭의 동일 요청은 5초 캐시로 합친다. 설정을 편집하거나 파일을 가져오는 동안에는 **연결 테스트**를 누르기 전까지 API를 호출하지 않는다.

애드온은 최대 24개, 입력·헤더·표시 항목은 각각 16개, 정의 파일은 48KiB다. 입력값은 항목당 8KiB, POST 본문은 템플릿 적용 전 16KiB·적용 후 64KiB, 저장·테스트 요청 전체는 64KiB까지 받는다. API 응답은 1MiB·6초, 동시 API 요청은 최대 4개로 제한한다.

API 계약은 [openapi.json](openapi.json)의 `/addons` 경로와 `AddonManifest` 스키마를 참고한다. 설치·테스트·조회 실행·변경·이동·삭제는 로그인 세션과 CSRF를 요구한다. 변경·이동·삭제에는 `If-Match` revision도 검사한다.

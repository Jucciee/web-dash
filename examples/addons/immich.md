# Immich 애드온

[immich.web-dash.json](immich.web-dash.json)을 가져오면 Immich 서버 전체의 **사진 수·동영상 수·미디어 사용량**을 표시한다. 기본 갱신 간격은 홈을 보고 있을 때 1분이다.

## Immich API 키 만들기

1. Immich 웹에 **관리자 계정**으로 로그인한다.
2. 오른쪽 위 사용자 메뉴에서 **계정 설정(Account Settings) → API Keys**를 연다. API 키는 사용자 설정에서 생성한다. [Immich 사용자 설정 안내](https://docs.immich.app/features/user-settings/)
3. 이름을 `web-dash`로 정하고 **`server.statistics` 권한**을 선택해 API 키를 만든다. 서버 통계 경로는 관리자 계정과 해당 권한을 요구한다. [공식 통계 API 구현](https://github.com/immich-app/immich/blob/main/server/src/controllers/server.controller.ts), [권한 정의](https://github.com/immich-app/immich/blob/main/server/src/enum.ts)
4. 생성한 키를 복사해 web-dash의 **내 연결값**에 입력한다. API 키별 권한을 지정할 수 있다. [공식 API 키 안내](https://docs.immich.app/features/command-line-interface/#obtain-the-api-key)

## web-dash에서 설치

1. **설정 → 애드온 → JSON 가져오기**에서 `immich.web-dash.json`을 선택한다.
2. **Immich 서버 주소 (/api 제외)**에 `https://photos.example.com` 또는 `http://192.168.1.10:2283`처럼 입력한다. 끝의 `/`, `/api`, `/photos`는 붙이지 않는다.
3. **Immich API 키**에 만든 키를 입력한다.
4. **연결 테스트**에서 HTTP 200과 세 지표를 확인하고 **애드온 설치**를 누른다.

web-dash를 Docker로 실행한다면 서버 주소는 web-dash 컨테이너에서 접근할 수 있어야 한다. Immich와 같은 Docker 네트워크에서는 `http://immich-server:2283`처럼 실제 서비스 이름을 사용할 수 있다. `localhost`는 web-dash가 실행되는 환경을 가리킨다.

## 요청과 표시 규칙

`GET /api/server/statistics`를 호출하며 `x-api-key` 헤더로 입력한 키를 전달한다. [Immich 통계 API](https://github.com/immich-app/immich/blob/main/server/src/controllers/server.controller.ts), [공식 인증 헤더 예제](https://docs.immich.app/guides/python-file-upload/)

| 지표 | 응답 경로 | 표시 예 |
| --- | --- | --- |
| 사진 | `/photos` | `1200장` |
| 동영상 | `/videos` | `80개` |
| 사용량 | `/usage` | `15.0 GiB` |

`usage`는 통계 API가 반환하는 바이트 단위의 미디어 사용량이다. 서버 디스크의 전체 사용률이나 남은 공간은 다른 지표다. 응답 필드는 공식 `ServerStatsResponseDto`를 기준으로 구성했다. [응답 형식](https://github.com/immich-app/immich/blob/main/server/src/dtos/server.dto.ts)

표시 예의 수치는 설명용이다. 실제 Immich 서버에 연결하면 해당 서버의 응답값을 표시한다. 서버 주소와 API 키는 파일에 들어 있지 않고 설치할 때 입력한다. 키는 암호화해 저장하며 내보내기에 포함하지 않는다.

## 연결이 안 될 때

- **401:** API 키가 비어 있거나 잘못됐는지, 삭제된 키인지 확인한다.
- **403:** 키를 만든 계정이 관리자이고 `server.statistics` 권한이 있는지 확인한다.
- **404:** 입력 주소에 `/api`를 중복해서 붙였는지 확인한다. 이 예제가 `/api/server/statistics`를 자동으로 붙인다.
- **3xx:** 이동 후의 최종 Immich 주소를 입력한다. 애드온은 인증값을 다른 주소로 전달하는 리다이렉트를 따라가지 않는다.
- **연결 실패:** web-dash 서버에서 해당 주소·포트에 접근할 수 있는지, HTTPS 인증서가 유효한지 확인한다.

2026-10-10 확인한 Immich 공식 API를 기준으로 작성했다. 사용 중인 Immich의 버전이나 프록시 설정에 따라 API 응답·접근 조건이 달라질 수 있다.

# API 계약 초안

기본 경로는 `/api/v1`이다. 이 문서는 구현할 계약의 초안이며, 첫 작업에서 같은 내용을 OpenAPI로 옮겨 타입·문서 생성을 연결한다.

2026-10-03 구현 계약은 [openapi.json](openapi.json)이 기준이다. 현재 가져오기 preview는 JSON `{dashboardId, kind, sourceName, entryPath, files: [{path, content}]}`를 받는다. Docker 폴더 검색은 `POST /imports/docker/scan`에 `{dashboardId, folder, publicHost}`를 보낸다. 후보의 diff는 new·duplicate이며 commit은 후보 ID와 편집한 ServiceInput을 선택 등록한다. 아래 multipart·출처 snapshot·update/link·변경 비교는 후속 확장안이다.

## 1. 공통 규칙

- JSON 필드는 camelCase를 사용한다. ID는 서버에서 생성한 UUID 문자열을 기본으로 한다.
- 시간은 UTC RFC 3339 문자열, 알 수 없는 값은 `null`로 반환한다.
- 공개 경로는 초기 상태 확인·초기 계정 생성·로그인·CSRF 토큰 조회와 process health다. 나머지 관리 API는 세션 인증을 요구한다.
- 브라우저의 변경 요청에는 `X-CSRF-Token`을 요구한다. 호스트 지표 수신은 별도 Bearer token 인증이다.
- 입력 크기 제한을 적용한다. 카드 URL은 `http` 또는 `https`만 허용한다.
- 저장 변경은 revision을 검사한다. 상태 조회는 저장 revision을 변경하지 않는다.
- 외부 오류는 원본 비밀정보를 포함하지 않는 오류 code로 변환한다.

성공 응답은 다음 형태다. 삭제·로그아웃처럼 본문이 없는 성공은204를 사용한다.

```json
{
  "data": {},
  "meta": { "requestId": "request-example" }
}
```

```json
{
  "error": {
    "code": "IMPORT_MISSING_FILE",
    "message": "참조한 설정 파일을 함께 올려주세요.",
    "details": { "file": "sites/photos.conf" }
  },
  "meta": { "requestId": "request-example" }
}
```

주요 상태 코드는400 입력 오류, 401 인증 필요, 403 권한·CSRF 실패, 404 없음, 409 revision·중복 적용 충돌, 413 크기 제한, 422 분석 불가, 429 요청 제한, 502 외부 연결 실패, 504 외부 timeout이다.

## 2. 초기 설정·인증

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| GET | `/setup/status` | `needsSetup`만 공개 |
| GET | `/auth/csrf` | 로그인·초기 설정에 사용할 CSRF 토큰 반환 |
| POST | `/setup` | setupToken·username·password로 최초 관리자 생성 |
| POST | `/sessions` | 로그인, 세션 쿠키 설정 |
| GET | `/me` | 사용자·CSRF 토큰 조회 |
| DELETE | `/sessions/current` | 로그아웃, 세션 폐기 |
| GET | `/health/live` | 프로세스 생존 여부만 반환 |

초기 토큰은 설치 시 발급하고 사용 후 폐기한다. 이미 관리자가 있으면 setup API를 거절한다. 관리자가 동시에 생성되지 않도록 DB 제약과 트랜잭션으로 검사한다. 로그인은 rate limit을 적용한다.

## 3. 대시보드·그룹·카드

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| GET | `/dashboards` | 기본 대시보드 목록 |
| GET | `/dashboards/{id}` | 그룹·카드·layout revision |
| POST | `/dashboards/{id}/groups` | 그룹 생성 |
| PATCH | `/groups/{id}` | 그룹 이름 수정 |
| DELETE | `/groups/{id}` | 비어 있는 그룹 삭제, 카드가 있으면409 |
| GET | `/services?dashboardId={id}` | 카드 목록 |
| POST | `/services` | 카드 생성 |
| PATCH | `/services/{id}` | 카드 수정 |
| DELETE | `/services/{id}` | 카드와 대시보드 내부 관계 삭제 |
| PUT | `/dashboards/{id}/layout` | 그룹·카드 순서 일괄 저장 |
| GET | `/dashboards/{id}/statuses` | 캐시된 카드별 웹·Docker 상태 |

카드 생성 요청 예시다. 그룹을 통해 대시보드가 정해진다.

```json
{
  "groupId": "group-example",
  "name": "사진 서버",
  "url": "https://photos.example.com/",
  "iconKey": "photos",
  "description": "개인 사진 보관",
  "hidden": false
}
```

카드 응답에는 위 필드 외에 `id`, `position`, `revision`, `createdAt`, `updatedAt`을 추가한다. PATCH·DELETE는 `If-Match: "{revision}"`을 검사한다. 그룹에도 revision을 둔다.

layout 요청은 `If-Match`에 대시보드 revision을 사용하고, 대시보드의 모든 그룹·카드 ID를 정확히 한 번씩 포함한다. hidden 카드도 포함한다.

```json
{
  "groups": [
    { "groupId": "group-example", "serviceIds": ["service-a", "service-b"] },
    { "groupId": "group-other", "serviceIds": [] }
  ]
}
```

구성원 ID·중복·소유 대시보드·revision을 검사한다. 그룹·카드 생성·삭제·그룹 이동과 layout 변경은 대시보드 revision을 증가시킨다. 카드 내용 변경은 카드 revision을 증가시키며, 순서 변경으로 position이 바뀐 카드도 카드 revision을 증가시킨다.

## 4. 가져오기 preview

`POST /imports/preview`는 `multipart/form-data`를 받는다.

| 필드 | 설명 |
| --- | --- |
| `kind` | `nginx` 또는 `caddy` |
| `sourceName` | 출처 표시 이름 |
| `sourceId` | 재가져오기면 기존 출처 ID, 신규면 생략 |
| `entryPath` | 분석 시작 파일의 가상 경로 |
| `manifest` | 각 파일의 가상 경로와 multipart 필드명의 JSON 대응표 |
| `file0`, `file1` 등 | 파일 내용, 실제 서버 경로로 해석하지 않음 |

붙여넣기는 UI가 내용을 파일처럼 묶어 같은 API로 보낸다. `nginx -T` 출력이면 manifest에 입력 형식을 표시해 file section을 복원한다. 초기 제한값은 개별 파일2MiB, 총10MiB, 파일100개, preview 유효시간30분으로 제안한다.

응답 예시는 `data` 안에 다음 값을 담는다. 예시 ID는 실제 구현에서 서버 생성 UUID를 사용한다.

```json
{
  "importId": "import-example",
  "dashboardRevision": 7,
  "expiresAt": "2026-10-03T10:30:00Z",
  "candidates": [
    {
      "id": "candidate-example",
      "sourceKey": "photos.example.com/",
      "suggestedName": "photos",
      "suggestedUrl": "https://photos.example.com/",
      "upstreams": ["photos:3000"],
      "file": "Caddyfile",
      "line": 1,
      "diff": "new",
      "existingServiceId": null,
      "warnings": []
    }
  ],
  "warnings": []
}
```

diff 값은 new, changed, unchanged, missing, ambiguous로 정의한다. missing은 이전 snapshot에는 있었지만 이번에 발견하지 못한 항목이다. ambiguous는 기존 카드와의 관계를 확정하지 못한 항목이며 가능한 연결 후보를 UI에 반환한다.

## 5. 가져오기 적용

`POST /imports/{id}/commit`는 JSON을 받는다. session 소유자·유효기간·dashboard revision을 검사하고 같은 요청의 재전송을 구분하도록 Idempotency-Key를 요구한다. 재가져오기면 preview 때의 source snapshot도 변경되지 않았는지 검사한다.

```json
{
  "dashboardRevision": 7,
  "items": [
    {
      "candidateId": "candidate-example",
      "action": "create",
      "name": "사진 서버",
      "url": "https://photos.example.com/",
      "groupId": "group-example",
      "iconKey": "photos"
    }
  ]
}
```

action은 create, update, link, skip이다. update와 link에는 serviceId와 serviceRevision도 필요하다. update는 사용자가 확인한 카드 필드만 변경하고 link는 원본과의 관계만 추가한다. 원본에서 사라진 카드 삭제는 일반 카드 삭제 API로 별도 처리한다.

후보ID 중복, URL 형식, 그룹 소속, service revision을 검증한다. 성공 시 serviceIds와 갱신된 dashboardRevision을 반환한다. 전체 항목은 한 트랜잭션으로 적용한다.

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| GET | `/imports/{id}` | 저장한 미리보기 조회 |
| GET | `/sources` | 가져오기 출처 목록 |
| DELETE | `/sources/{id}` | 출처 관계 해제, 카드 유지 |

## 6. Docker 연결·조회

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| GET | `/docker/connections` | 연결 목록, 민감정보 제외 |
| POST | `/docker/connections` | Dashboard 내부 연결 설정 저장 |
| PATCH | `/docker/connections/{id}` | 연결 설정 변경 |
| DELETE | `/docker/connections/{id}` | 연결 설정·binding 제거, 카드 유지 |
| POST | `/docker/connections/{id}/test` | Docker의 ping·version을 읽어 연결 검사 |
| GET | `/docker/connections/{id}/containers` | 컨테이너 목록·상태·health |
| GET | `/docker/connections/{id}/containers/{containerId}/stats` | 캐시된 CPU·RAM |
| PUT | `/services/{id}/docker-binding` | 카드의 컨테이너 선택 설정 저장 |
| DELETE | `/services/{id}/docker-binding` | 카드의 상태 연결 해제 |

위 POST·PATCH·DELETE는 대시보드 설정에 대한 작업이다. Docker 데몬으로 나가는 요청은 허용된 조회와 접속 검사뿐이다. Docker 변경용 endpoint는 정의하지 않는다.

컨테이너 연결은 Compose project/service 또는 이름 selector를 저장한다. 현재 선택된 container ID도 응답에 담되, 재생성되면 selector를 재해석한다. 여러 개가 일치하면 `binding_ambiguous`로 표시한다.

## 7. 상태 DTO

`GET /dashboards/{id}/statuses`의 `data`는 다음 객체들의 배열이다.

```json
{
  "serviceId": "service-example",
  "observedAt": "2026-10-03T10:00:00Z",
  "stale": false,
  "web": {
    "state": "ok",
    "httpStatus": 200,
    "observedAt": "2026-10-03T10:00:00Z"
  },
  "docker": {
    "containerId": "hex-id-example",
    "state": "running",
    "health": "healthy",
    "cpuPercent": 2.4,
    "memoryBytes": 125829120,
    "limitBytes": null,
    "observedAt": "2026-10-03T10:00:00Z"
  },
  "errorCode": null
}
```

최상위 observedAt은 가장 최근 관측 시각이며 web과 docker도 각각 관측 시각을 가진다. 한쪽이 오래된 경우 개별 시각으로 표시한다. 대상 관측 중 하나라도 기준을 초과하면 stale을 true로 한다. Docker 미연결은 docker:null, 연결이 끊긴 경우는 마지막 값과 stale을 반환한다.

web state는 ok, auth_required, error, unknown, disabled다. 초기 HTTP 판정은 2xx/3xx를 응답 있음, 401/403을 인증 필요, 나머지를 오류로 처리하고 redirect 수를 제한한다. 인증이 필요하다고 서비스를 정지 상태로 표시하지 않는다. health는 healthy/unhealthy/starting/none이며 아직 읽지 못한 경우 null이다.

## 8. 호스트 수집기

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| POST | `/hosts` | 수집기 등록과 한 번 표시하는 token 발급 |
| GET | `/hosts` | 연결 상태와 마지막 수신 시각 |
| DELETE | `/hosts/{id}` | 수집기 token 폐기와 연결 제거 |
| POST | `/hosts/{id}/samples` | 수집기의 별도 token으로 지표 전송 |
| GET | `/hosts/{id}/metrics` | 현재값·제한된 최근 추세 조회 |

samples에는 capturedAt, cpuPercent, memoryTotalBytes, memoryAvailableBytes, disks 배열과 collector OS 정보를 넣는다. 음수, 비정상 시각, 과도한 payload와 전송 빈도는 거절한다. token은 hash로 저장하고 관리 화면에 다시 표시하지 않는다.

## 9. 설정·백업

| 메서드 | 경로 | 동작 |
| --- | --- | --- |
| GET | `/settings` | 표시 설정 |
| PATCH | `/settings` | 테마·표시 설정 변경 |
| GET | `/backups/export` | version이 있는 비밀정보 제외 JSON 다운로드 |
| POST | `/backups/preview` | 복원 파일 검사와 변경 미리보기 |
| POST | `/backups/{restoreSessionId}/commit` | revision 검사 후 트랜잭션 복원 |

첫 버전의 복원은 기존 카드·그룹·배치·표시 설정을 교체하는 방식이다. Docker 연결과 수집기 인증은 별도 재설정한다. restore session에도 소유자·유효기간·재전송 구분을 적용한다.

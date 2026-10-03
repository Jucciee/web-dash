# 프론트엔드 구현 가이드

## 1. 프론트엔드 구성

React + TypeScript + Vite를 사용한다. 이 앱은 로그인 후 카드 편집과 상태 조회를 주로 하므로 SPA로 구성하고, Go가 빌드 파일을 제공한다.

React Router로 화면을 나누고 TanStack Query로 서버 데이터를 관리한다. 스타일은 CSS Modules와 공통 CSS 변수로 시작한다. 폼은 처음에는 React state와 작은 검증 함수로 만들고, 조건부 필드가 많아질 때 React Hook Form을 검토한다. dnd kit은 카드·그룹 정렬 단계에서 도입한다. [dnd kit 공식 문서](https://dndkit.com/)

## 2. 화면과 URL

| 경로 | 주요 화면 |
| --- | --- |
| `/setup` | 초기 토큰 입력, 관리자 생성 |
| `/login` | 로그인 |
| `/` | 그룹·카드 대시보드와 상태 요약 |
| `/imports/new` | Nginx·Caddy 선택, 파일 업로드·붙여넣기 |
| `/imports/:id` | 후보 확인·수정·선택 등록·변경 비교 |
| `/connections` | Docker 연결 목록·검사·컨테이너 조회 |
| `/hosts` | 호스트 수집기 등록과 CPU·RAM·디스크 |
| `/settings` | 테마·백업·복원·기본 표시 설정 |

카드 생성과 수정은 대시보드의 dialog로 제공한다. 자세한 상태는 side panel로 열어 대시보드 위치를 유지한다. 좁은 화면에서는 전체 화면 dialog로 바꾼다.

## 3. 기본 화면 배치

```text
┌─────────────────────────────────────────────────────────┐
│ 내 홈서버        검색             서비스 추가  가져오기 │
├─────────────────────────────────────────────────────────┤
│ 호스트 CPU · RAM · 디스크     Docker 연결 상태           │
├─────────────────────────────────────────────────────────┤
│ 미디어                              편집                │
│ [사진 서버] [동영상 서버] [파일 서버]                     │
│ 각 카드: 이름 / 웹 응답 상태 / Docker 상태 / 사용량       │
│                                                         │
│ 관리                                                    │
│ [서버 관리] [네트워크 관리]                              │
└─────────────────────────────────────────────────────────┘
```

처음에는 읽기 쉬운 카드 목록과 그룹 순서만 구현한다. 화면 상단에는 자주 쓰는 추가·가져오기를 둔다. 연결 세부사항은 설정 화면에서 설명한다. 모바일에서 접속 URL을 여는 영역과 편집 버튼이 겹치지 않게 한다.

## 4. 컴포넌트와 폴더

```text
web/src/
├─ app/                  # Router, QueryProvider, 인증 경계
├─ api/                  # fetch wrapper, DTO 타입
├─ features/
│  ├─ dashboard/         # DashboardPage, ServiceCard, GroupSection
│  ├─ services/          # ServiceEditorDialog, ContainerBindingPicker
│  ├─ imports/           # ImportInput, CandidateList, DiffView, CommitBar
│  ├─ connections/       # ConnectionForm, ConnectionStatus, ContainerTable
│  ├─ hosts/             # HostSummary, MetricChart, CollectorSetup
│  ├─ auth/              # SetupPage, LoginPage
│  └─ settings/          # ThemeForm, BackupRestore
├─ components/           # Button, Dialog, StatusBadge, Field, EmptyState
├─ styles/               # 토큰, 전역 스타일
└─ main.tsx
```

feature별 API hook과 화면을 가까이 둔다. 버튼·dialog처럼 여러 화면에서 쓰는 요소만 공통 폴더로 옮긴다. 데이터 변환과 화면 표시를 작은 함수로 분리해 복잡한 JSX 조건문을 줄인다.

## 5. 상태 관리

| 상태 | 보관 위치 |
| --- | --- |
| 카드·그룹·Docker 상태 | TanStack Query 캐시 |
| 가져오기 후보와 저장 상태 | 서버의 import session + Query 캐시 |
| 후보의 수정 중인 이름·URL·선택 여부 | 가져오기 화면의 local state |
| dialog 열림·검색어·편집 모드 | 컴포넌트 state 또는 URL |
| 로그인 세션 | 서버의 HttpOnly cookie |
| 테마 | 서버 설정, 화면 깜박임 방지용 비민감 캐시 가능 |

초기에는 별도 전역 상태 라이브러리를 넣지 않는다. Query 데이터와 같은 값을 다른 저장소에 복제하면 동기화가 어려워진다.

서버의 Docker 캐시는 모든 클라이언트가 공유한다. 브라우저는 기본 5초 간격으로 캐시를 조회하며 숨긴 탭에서는 주기 조회를 멈춘다. 필요한 시점에 SSE를 추가할 수 있지만 첫 버전의 구현 조건은 아니다.

## 6. API 타입·조회 예시

```ts
export type ServiceStatus = {
  serviceId: string;
  observedAt: string | null;
  stale: boolean;
  web: {
    state: "ok" | "auth_required" | "error" | "unknown" | "disabled";
    httpStatus: number | null;
  };
  docker: {
    state: string | null;
    health: "healthy" | "unhealthy" | "starting" | "none" | null;
    cpuPercent: number | null;
    memoryBytes: number | null;
  } | null;
};

export async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin" });
  if (!response.ok) {
    // 실제 구현에서는 API 오류 code와 requestId도 보관한다.
    throw new Error(`요청 실패: ${response.status}`);
  }
  const body = await response.json() as { data: T };
  return body.data;
}
```

```tsx
const statusQuery = useQuery({
  queryKey: ["service-statuses", dashboardId],
  queryFn: () => getJSON<ServiceStatus[]>(
    `/api/v1/dashboards/${encodeURIComponent(dashboardId)}/statuses`
  ),
  refetchInterval: 5000,
  refetchIntervalInBackground: false,
  staleTime: 3000,
});
```

배포에서는 같은 origin의 `/api`를 사용한다. 변경 fetch에는 서버에서 받은 CSRF 토큰을 `X-CSRF-Token`에 넣는다. 401이면 로그인으로 안내하고, 409이면 최신 값을 불러와 변경 내용을 다시 확인하게 한다. 204 응답에는 JSON parsing을 하지 않는다.

## 7. 가져오기 화면 구현

후보 행에는 선택 checkbox, 이름, URL, 그룹, 발견 출처, 경고를 표시한다. 수정한 후보는 local state에 두고 `등록`을 누를 때 한 번에 commit한다. 미확정 URL은 빈 입력칸과 해결 안내를 보여주고 입력 전에는 해당 후보를 등록할 수 없게 한다.

후보가 많으면 검색·전체 선택·경고 있는 항목 필터를 제공한다. commit 중 중복 클릭을 막고, 네트워크 재시도는 같은 import session과 idempotency key로 처리한다. 단순 업로드 분석 실패와 등록 실패를 다른 메시지로 안내한다.

source 후보와 Docker 컨테이너를 자동 연결할 때는 추천 근거를 보여준다. 예: `프록시 대상 photos:3000과 컨테이너 이름 일치`. 다수 컨테이너와 일치하면 사용자가 선택하게 한다.

## 8. 사용성·접근성

- 상태는 색뿐 아니라 텍스트와 아이콘으로 표현한다.
- loading 동안 카드 전체를 매번 비우지 않고 마지막 값을 유지한다.
- 연결 실패와 컨테이너 정지를 구분한다. healthcheck 없음도 별도로 표시한다.
- form 필드에 label, 오류 설명, focus 이동을 제공한다.
- dialog를 닫으면 원래 버튼으로 focus를 돌린다.
- 카드 클릭 영역에는 편집 버튼을 중첩하지 않는다.
- 드래그에는 키보드 대체 동작을 제공하고 작은 화면에서는 순서 이동 버튼을 쓴다.
- 360px 폭에서 업로드·후보 수정·로그인·카드 이동을 확인한다.

아이콘은 처음에는 제공된 icon key와 업로드한 이미지로 제한한다. 임의 원격 주소를 서버에서 자동 fetch하는 기능은 추가하지 않는다. 업로드는 크기·형식 검사와 실제 이미지 decode를 수행하고, 사용자 SVG는 첫 버전에서 허용하지 않는다.

## 9. 프론트 구현·검증 순서

mock API 데이터로 대시보드·가져오기 화면부터 만들고 API 계약이 안정되면 Go 서버와 연결한다. 카드 CRUD, 후보 선택 등록, 상태 끊김 복구, 배치 저장을 실제 서버에서 확인한다.

검증은 Vitest로 상태 표시·후보 편집 같은 중요한 로직을 확인하고, Playwright로 `로그인 → 가져오기 → 선택 등록 → 카드 편집 → 재접속`의 대표 흐름을 확인한다. 모든 컴포넌트의 모양을 복제하는 테스트는 만들지 않는다.

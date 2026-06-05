# SDD — MagicStrike Web Frontend

## 1. Visão Geral

Frontend SPA para **MagicStrike** — plataforma de análise tática inteligente de replays de CS2.
Construído com React 18 + TypeScript + Vite + Tailwind CSS, tema preto e branco (cursor-ide).

### Stack Tecnológica

| Camada | Tecnologia | Justificativa |
|:---|:---|:---|
| Framework | React 18 | Ecossistema maduro, ampla comunidade |
| Linguagem | TypeScript 5 (strict) | Segurança de tipos, melhor DX |
| Bundler | Vite 6 | HMR rápido, build otimizado |
| Estilo | Tailwind CSS 4 | Utility-first, tema escuro por padrão |
| Roteamento | React Router v7 | SPA routing com loaders/actions |
| HTTP Client | Axios | Interceptors para JWT, upload progress |
| Validação | Valibot | Validacão de formulários, tipagem inferida |
| Ícones | Lucide React | Ícones consistentes, tree-shakeable |
| State | React Context + useReducer | Auth state, chat state (sem overkill de Redux) |

### Estrutura de Pastas

```
src/
├── api/                  # Cliente HTTP e endpoints
│   ├── client.ts         # Axios instance + interceptors
│   ├── auth.ts           # POST /auth/magic-link, /auth/callback, /auth/refresh, DELETE /auth/session
│   ├── demos.ts          # POST /demos/upload-request, /demos/upload-confirm
│   ├── matches.ts        # GET /matches, GET /matches/:id
│   └── chat.ts           # GET /chat, POST /chat, GET /chat/:id, POST /chat/:id, DELETE /chat/:id
├── components/           # Componentes reutilizáveis
│   ├── ui/               # Design system (Button, Input, Card, Modal, Badge, Spinner, etc.)
│   └── layout/           # Shell, Navbar, Sidebar
├── contexts/             # React contexts
│   ├── AuthContext.tsx    # Autenticação: login, logout, refresh, user state
│   └── ChatContext.tsx    # Estado do chat atual
├── hooks/                # Custom hooks
│   ├── useAuth.ts
│   ├── useMatches.ts
│   └── useChat.ts
├── pages/                # Páginas (route-level components)
│   ├── Login.tsx          # Tela de login com magic link
│   ├── Callback.tsx       # Processa token do magic link
│   ├── Dashboard.tsx      # Lista de partidas (home)
│   ├── MatchDetail.tsx    # Detalhes de uma partida
│   ├── Upload.tsx         # Upload de demo
│   ├── ChatList.tsx       # Lista de chats
│   └── ChatRoom.tsx       # Conversa individual
├── lib/                  # Utilitários
│   ├── storage.ts         # localStorage wrapper com tipagem
│   └── format.ts          # Formatação de datas, scores, etc.
├── App.tsx               # Rotas e providers
├── main.tsx              # Entry point
└── index.css             # Tailwind directives + tema
```

---

## 2. Design System — Tema Cursor IDE (Preto e Branco)

### Paleta Monocromática

```
┌─────────────────────────────────────────────────────────┐
│  Cursor IDE Theme — Black & White                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  background        #0D0D0D  (bg-primary)                │
│  surface           #1A1A1A  (bg-secondary)              │
│  surface-hover     #252525  (bg-tertiary)               │
│  border            #333333  (border)                     │
│  border-focus      #555555  (border-accent)             │
│  text-primary      #F0F0F0  (text)                       │
│  text-secondary    #999999  (text-muted)                 │
│  text-tertiary     #666666  (text-dim)                   │
│  accent            #FFFFFF  (accent-white)               │
│  accent-hover      #E0E0E0  (accent-hover)              │
│  success           #4ADE80  (green-400)                  │
│  error             #F87171  (red-400)                    │
│  warning           #FBBF24  (amber-400)                  │
│  info              #60A5FA  (blue-400)                   │
└─────────────────────────────────────────────────────────┘
```

### Tipografia

- **Font**: Inter (sans-serif) — mesma do Cursor IDE
- **Escala**: text-xs (12px), text-sm (14px), text-base (16px), text-lg (18px), text-xl (20px), text-2xl (24px), text-3xl (30px)
- **Pesos**: normal (400), medium (500), semibold (600), bold (700)
- **Mono**: JetBrains Mono para status codes, IDs, dados técnicos

### Componentes Base (`src/components/ui/`)

```
Button        → variant: primary | secondary | ghost | danger
               → size: sm | md | lg
               → loading: boolean

Input         → label, error, icon (esquerda), type (text/email/password)

Card          → padding: sm | md | lg
               → hover: boolean (elevação sutil)

Badge         → variant: success | error | warning | info | neutral

Spinner       → size: sm | md | lg

Modal         → title, onClose, children

StatusDot     → status: pending | processing | processed | failed
               (animação pulse para pending/processing)

EmptyState    → icon, title, description, action (button opcional)

Toast         → type: success | error | info
               → position: top-right
```

---

## 3. Rotas e Árvore de Componentes

### Tabela de Rotas

| Path | Página | Auth | Descrição |
|:---|:---|:---|:---|
| `/login` | Login | Não | Formulário de magic link |
| `/auth/callback?token=` | Callback | Não | Processa token e redireciona |
| `/` | Dashboard | Sim | Lista de partidas do usuário |
| `/matches/:id` | MatchDetail | Sim | Detalhes de uma partida |
| `/upload` | Upload | Sim | Upload de demo (.dem) |
| `/chat` | ChatList | Sim | Lista de sessões de chat |
| `/chat/:id` | ChatRoom | Sim | Conversa com IA |

### Árvore de Componentes

```
<App>
  <AuthProvider>
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/auth/callback" element={<Callback />} />

        {/* Rotas protegidas */}
        <Route element={<ProtectedRoute />}>
          <Route element={<AppShell />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/matches/:id" element={<MatchDetail />} />
            <Route path="/upload" element={<Upload />} />
            <Route path="/chat" element={<ChatList />} />
            <Route path="/chat/:id" element={<ChatRoom />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  </AuthProvider>
</App>
```

### AppShell Layout

```
┌─────────────────────────────────────────────────────┐
│  Navbar (fixo topo)                                  │
│  ┌─────────────────────────────────────────────────┐│
│  │  [🔮 MagicStrike]     [Matches] [Upload] [Chat] ││
│  │                                      [👤 User]  ││
│  └─────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────┤
│                                                      │
│  Conteúdo da página (scroll)                         │
│                                                      │
│                                                      │
├─────────────────────────────────────────────────────┤
│  Status bar (fixo rodapé) — "Connected to API"      │
└─────────────────────────────────────────────────────┘
```

---

## 4. API Client — Design

### Axios Instance (`src/api/client.ts`)

```typescript
// Configuração base
const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  headers: { 'Content-Type': 'application/json' },
  timeout: 30000,
});

// Request interceptor — injeta JWT
api.interceptors.request.use((config) => {
  const token = storage.getToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Response interceptor — trata 401 com refresh automático
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401 && !error.config._retry) {
      error.config._retry = true;
      const newToken = await refreshToken();
      if (newToken) {
        error.config.headers.Authorization = `Bearer ${newToken}`;
        return api(error.config);
      }
      // Refresh falhou → logout
      storage.clear();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
```

### Tratamento de Erros

Todo erro da API é normalizado para:
```typescript
type ApiError = {
  type: string;      // RFC 7807: "about:blank"
  title: string;
  status: number;
  detail: string;
  instance?: string;
}
```

---

## 5. Fluxo de Autenticação

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  /login  │ ──▶ │  Email   │ ──▶ │ 202 OK   │ ──▶ │ "Check   │
│          │     │  form    │     │          │     │  email"  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                                                         │
                                                    (em dev: token
                                                     nos logs)
                                                         │
                                                         ▼
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│Dashboard │ ◀── │ Store    │ ◀── │ 200 OK   │ ◀── │/callback │
│   /      │     │ JWT+user │     │          │     │?token=.. │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
```

---

## 6. Páginas — Especificação Detalhada

### 6.1 Login (`/login`)

**Estados:**
- **idle**: Formulário de email vazio
- **loading**: Spinner no botão após submit
- **success**: Mensagem verde "Magic link enviado! Verifique seu email"
- **error**: Erro de validação (email inválido) ou rede

**Componentes:**
- Input email com validação Valibot
- Button "Send Magic Link"
- Status message condicional

### 6.2 Callback (`/auth/callback?token=`)

**Comportamento:**
- Extrai `token` da query string
- Chama `POST /auth/callback { token }`
- Em sucesso: armazena JWT + user no localStorage, redireciona para `/`
- Em erro (401): mostra mensagem "Link expirado ou inválido", link para `/login`

### 6.3 Dashboard (`/`)

**Estados:**
- **loading**: Skeleton cards (3 placeholders)
- **empty**: EmptyState "Nenhuma partida enviada ainda" + botão "Upload Demo"
- **data**: Grid de cards de partida
- **error**: Mensagem de erro com botão retry

**Cada Card de Partida mostra:**
```
┌──────────────────────────────┐
│  MIBR 13 — 6 Lynn Vision    │
│  de_anubis · 19 rounds      │
│  ┌────────────────────────┐ │
│  │ status: ● processed    │ │
│  └────────────────────────┘ │
│  Enviado em 05/06/2026      │
└──────────────────────────────┘
```

**Paginação:** Botões "Anterior" / "Próximo" com offset/limit.

### 6.4 Upload (`/upload`)

**Fluxo em 3 passos visuais:**

```
Step 1: Fill metadata        Step 2: Upload file        Step 3: Processing
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│ Team A: [____]   │    │ ⬆ Uploading...   │    │ ✅ Queued!       │
│ Team B: [____]   │──▶│ ████████░░ 75%   │──▶│                  │
│ Map: [select]    │    │                  │    │ View match →    │
│ File: [.dem]     │    │                  │    │                  │
└──────────────────┘    └──────────────────┘    └──────────────────┘
```

### 6.5 Chat (`/chat` e `/chat/:id`)

**ChatList (`/chat`):**
- Lista de sessões de chat existentes
- Botão "New Chat" — modal para selecionar matches e fazer primeira pergunta
- Cada item: última pergunta, match count, timestamp

**ChatRoom (`/chat/:id`):**
- Layout estilo chat (bolhas de pergunta/resposta)
- Input de pergunta no rodapé (max 500 chars, contador)
- Data points exibidos como badges abaixo das respostas
- Scroll automático para última mensagem
- Indicador de source (ClickHouse/Qdrant)

---

## 7. Estratégia de Estado

### AuthContext

```typescript
type AuthState = {
  user: User | null;
  token: string | null;
  sessionId: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;  // true durante restore do localStorage
};

type AuthActions = {
  login: (email: string) => Promise<void>;
  handleCallback: (token: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshToken: () => Promise<void>;
};
```

### Persistência

- `magicstrike_token` → JWT access token
- `magicstrike_user` → User object (JSON)
- `magicstrike_session` → Session ID

---

## 8. Plano de Testes

| Tipo | Escopo | Ferramenta |
|:---|:---|:---|
| Unit | Componentes UI, hooks, validação | Vitest + Testing Library |
| Integration | Fluxos completos (login → dashboard → chat) | Vitest + MSW |
| E2E | Smoke test dos fluxos críticos | Playwright |

---

## 9. Deploy

- Build: `vite build` → `dist/`
- Serve: Nginx com SPA fallback (`try_files $uri /index.html`)
- Variáveis de ambiente via `.env`: `VITE_API_BASE_URL`

# GitHub Activity Metrics

GitHub Organization の開発活動データを収集・集計し、開発チームおよび個人単位の開発生産性を可視化するための API および CLI ツールです。

## 機能

- GitHub Organization の全 Repository の活動データを収集
- Commit、Pull Request、コード変更量（追加・削除行数）、デプロイ情報を取得
- Organization / Repository / Member 単位でメトリクスを集計
- 時系列（日・週・月）でのデータ集計
- REST API によるデータ提供
- CLI コマンドによる可視化

## 必要条件

- Go 1.25 以上
- GitHub Personal Access Token（repo, read:org スコープが必要）

## インストール

```bash
# リポジトリをクローン
git clone https://github.com/kurihiro0119/github-activity-metrics.git
cd github-activity-metrics

# 依存関係をインストール
make setup

# ビルド
make build
```

## 設定

`.env.example` をコピーして `.env` ファイルを作成し、必要な設定を行います。

```bash
cp .env.example .env
```

### 環境変数

| 変数名         | 説明                                          | デフォルト値            |
| -------------- | --------------------------------------------- | ----------------------- |
| `GITHUB_TOKEN` | GitHub Personal Access Token                  | (必須)                  |
| `MODE`         | モード (`organization` または `user`)         | `organization`          |
| `STORAGE_TYPE` | ストレージタイプ (`sqlite` または `postgres`) | `sqlite`                |
| `SQLITE_PATH`  | SQLite データベースファイルのパス             | `./metrics.db`          |
| `POSTGRES_URL` | PostgreSQL 接続 URL                           | -                       |
| `API_PORT`     | API サーバーのポート                          | `8080`                  |
| `API_HOST`     | API サーバーのホスト                          | `localhost`             |
| `API_ENDPOINT` | CLI が使用する API エンドポイント             | `http://localhost:8080` |

## 使い方

### CLI

#### データ収集

```bash
# Organization のデータを収集（MODE=organization の場合）
./bin/github-metrics collect <org-name>

# 個人アカウントのデータを収集（MODE=user の場合）
./bin/github-metrics collect <username>

# 期間を指定して収集
./bin/github-metrics collect <org-name> --start 2024-01-01 --end 2024-12-31
```

**モードの切り替え:**

- 環境変数 `MODE=organization` で組織モード（デフォルト）
- 環境変数 `MODE=user` で個人アカウントモード

#### メトリクス表示

**Organization モード (`MODE=organization`):**

```bash
# Organization 全体のメトリクスを表示
./bin/github-metrics show <org-name>

# メンバー別メトリクスを表示
./bin/github-metrics show members <org-name>

# 特定メンバーのメトリクスを表示
./bin/github-metrics show member <org-name> <username>

# リポジトリ別メトリクスを表示
./bin/github-metrics show repos <org-name>

# 特定リポジトリのメトリクスを表示
./bin/github-metrics show repo <org-name> <repo-name>
```

**User モード (`MODE=user`):**

```bash
# ユーザー全体のメトリクスを表示
./bin/github-metrics show <username>

# コントリビューター別メトリクスを表示（フォークやコラボレーター含む）
./bin/github-metrics show members <username>

# 特定コントリビューターのメトリクスを表示
./bin/github-metrics show member <username> <contributor-username>

# リポジトリ別メトリクスを表示
./bin/github-metrics show repos <username>

# 特定リポジトリのメトリクスを表示
./bin/github-metrics show repo <username> <repo-name>
```

> **注意:** User モードでも、リポジトリにコントリビュートしたすべてのユーザー（フォークやコラボレーター含む）がメンバーとして識別されます。

#### オプション

```bash
--json          # JSON 形式で出力
--start         # 開始日 (YYYY-MM-DD)
--end           # 終了日 (YYYY-MM-DD)
--granularity   # 集計粒度 (day, week, month)
```

### API サーバー

```bash
# API サーバーを起動
make run-api
# または
./bin/github-metrics-api
```

#### API エンドポイント

**Organization エンドポイント:**
| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/health` | ヘルスチェック |
| GET | `/api/v1/orgs/:org/metrics` | Organization メトリクス |
| GET | `/api/v1/orgs/:org/metrics/timeseries` | 時系列メトリクス（単一メトリクスタイプ） |
| GET | `/api/v1/orgs/:org/metrics/timeseries/detailed` | 時系列メトリクス（詳細：全メトリクス含む） |
| GET | `/api/v1/orgs/:org/members/metrics` | 全メンバーメトリクス |
| GET | `/api/v1/orgs/:org/members/:member/metrics` | 特定メンバーメトリクス |
| GET | `/api/v1/orgs/:org/members/:member/metrics/timeseries` | 特定メンバーの時系列メトリクス |
| GET | `/api/v1/orgs/:org/repos/metrics` | 全リポジトリメトリクス |
| GET | `/api/v1/orgs/:org/repos/:repo/metrics` | 特定リポジトリメトリクス |
| GET | `/api/v1/orgs/:org/repos/:repo/metrics/timeseries` | 特定リポジトリの時系列メトリクス |
| GET | `/api/v1/orgs/:org/repos/:repo/members/metrics` | 特定リポジトリの全メンバーメトリクス |
| GET | `/api/v1/orgs/:org/rankings/members/:type` | メンバーランキング（期間指定可） |
| GET | `/api/v1/orgs/:org/rankings/repos/:type` | リポジトリランキング（期間指定可） |

**User エンドポイント:**
| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/api/v1/users/:user/metrics` | ユーザーメトリクス |
| GET | `/api/v1/users/:user/metrics/timeseries` | ユーザー時系列メトリクス（単一メトリクスタイプ） |
| GET | `/api/v1/users/:user/metrics/timeseries/detailed` | ユーザー時系列メトリクス（詳細：全メトリクス含む） |
| GET | `/api/v1/users/:user/repos/metrics` | 全リポジトリメトリクス |
| GET | `/api/v1/users/:user/repos/:repo/metrics` | 特定リポジトリメトリクス |
| GET | `/api/v1/users/:user/repos/:repo/metrics/timeseries` | 特定リポジトリの時系列メトリクス |
| GET | `/api/v1/users/:user/repos/:repo/members/metrics` | 特定リポジトリの全メンバーメトリクス |
| GET | `/api/v1/users/:user/rankings/members/:type` | メンバーランキング（期間指定可） |
| GET | `/api/v1/users/:user/rankings/repos/:type` | リポジトリランキング（期間指定可） |

#### クエリパラメータ

| パラメータ    | 説明                                            | デフォルト |
| ------------- | ----------------------------------------------- | ---------- |
| `start`       | 開始日 (YYYY-MM-DD)                             | 30 日前    |
| `end`         | 終了日 (YYYY-MM-DD)                             | 今日       |
| `granularity` | 集計粒度 (day, month)                           | day        |
| `type`        | メトリクスタイプ (commit, pull_request, deploy) | commit     |
| `limit`       | ランキング取得件数                              | 10         |

> **注意:** 時系列データ API (`/metrics/timeseries/detailed`, `/repos/:repo/metrics/timeseries`, `/members/:member/metrics/timeseries`) では、`granularity` は `day` または `month` のみサポートされています。

#### ランキングタイプ

ランキング API (`/rankings/members/:type`, `/rankings/repos/:type`) で使用可能なタイプ:

| タイプ         | 説明                                      |
| -------------- | ----------------------------------------- |
| `commits`      | Commit 数でランキング                     |
| `prs`          | Pull Request 数でランキング               |
| `code-changes` | コード変更量（追加+削除行数）でランキング |
| `deploys`      | デプロイ数でランキング                    |

#### ランキング API の使用例

```bash
# Commit数ランキング（過去30日、上位10件）
GET /api/v1/orgs/example-org/rankings/members/commits?limit=10

# PR数ランキング（期間指定、上位20件）
GET /api/v1/orgs/example-org/rankings/members/prs?start=2024-01-01&end=2024-12-31&limit=20

# コード変更量ランキング（過去7日、上位5件）
GET /api/v1/orgs/example-org/rankings/members/code-changes?start=2024-12-01&end=2024-12-07&limit=5

# リポジトリランキング（デプロイ数、期間指定）
GET /api/v1/orgs/example-org/rankings/repos/deploys?start=2024-11-01&end=2024-11-30&limit=10
```

#### レスポンス例

**Organization メトリクス:**

```json
{
  "data": {
    "org": "example-org",
    "total_repos": 50,
    "total_members": 20,
    "commits": 1234,
    "prs": 456,
    "additions": 50000,
    "deletions": 30000,
    "deploys": 100
  }
}
```

**メンバーランキング (commits):**

```json
{
  "data": [
    {
      "rank": 1,
      "member": "alice",
      "value": 234,
      "commits": 234,
      "prs": 45,
      "additions": 12000,
      "deletions": 8000,
      "deploys": 12
    },
    {
      "rank": 2,
      "member": "bob",
      "value": 189,
      "commits": 189,
      "prs": 32,
      "additions": 9800,
      "deletions": 6500,
      "deploys": 8
    }
  ]
}
```

**リポジトリランキング (prs):**

```json
{
  "data": [
    {
      "rank": 1,
      "repo": "frontend",
      "value": 156,
      "commits": 456,
      "prs": 156,
      "deploys": 45
    },
    {
      "rank": 2,
      "repo": "backend",
      "value": 134,
      "commits": 389,
      "prs": 134,
      "deploys": 38
    }
  ]
}
```

**時系列メトリクス (詳細):**

```json
{
  "data": {
    "granularity": "day",
    "dataPoints": [
      {
        "timestamp": "2025-10-07T00:00:00Z",
        "commits": 10,
        "prs": 5,
        "additions": 500,
        "deletions": 200,
        "deploys": 2
      },
      {
        "timestamp": "2025-10-08T00:00:00Z",
        "commits": 15,
        "prs": 8,
        "additions": 750,
        "deletions": 300,
        "deploys": 3
      }
    ]
  }
}
```

**特定リポジトリの全メンバーメトリクス:**

```json
{
  "data": [
    {
      "member": "alice",
      "commits": 50,
      "prs": 10,
      "additions": 5000,
      "deletions": 2000,
      "deploys": 5,
      "timeRange": {
        "start": "2024-01-01T00:00:00Z",
        "end": "2024-12-31T23:59:59Z",
        "granularity": "day"
      }
    },
    {
      "member": "bob",
      "commits": 30,
      "prs": 8,
      "additions": 3000,
      "deletions": 1500,
      "deploys": 3,
      "timeRange": {
        "start": "2024-01-01T00:00:00Z",
        "end": "2024-12-31T23:59:59Z",
        "granularity": "day"
      }
    }
  ]
}
```

#### 時系列データ API の使用例

```bash
# 組織全体の詳細時系列データ（日単位、過去30日）
GET /api/v1/orgs/example-org/metrics/timeseries/detailed?granularity=day

# 組織全体の詳細時系列データ（月単位、期間指定）
GET /api/v1/orgs/example-org/metrics/timeseries/detailed?start=2024-01-01&end=2024-12-31&granularity=month

# 特定リポジトリの時系列データ（日単位）
GET /api/v1/orgs/example-org/repos/frontend/metrics/timeseries?granularity=day

# 特定リポジトリの全メンバーメトリクス
GET /api/v1/orgs/example-org/repos/frontend/members/metrics?start=2024-01-01&end=2024-12-31

# 特定メンバーの時系列データ（月単位）
GET /api/v1/orgs/example-org/members/alice/metrics/timeseries?start=2024-01-01&end=2024-12-31&granularity=month

# ユーザーアカウントの詳細時系列データ
GET /api/v1/users/username/metrics/timeseries/detailed?granularity=day

# ユーザーの特定リポジトリの全メンバーメトリクス
GET /api/v1/users/username/repos/my-repo/members/metrics?start=2024-01-01&end=2024-12-31
```

## 開発

```bash
# テストを実行
make test

# リントを実行
make lint

# ビルド成果物を削除
make clean
```

## アーキテクチャ

```
┌─────────────────────────────────────────────────────────┐
│                    CLI (cobra)                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │         API Client (HTTP Client)                  │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          │ HTTP
                          ▼
┌─────────────────────────────────────────────────────────┐
│              API Server (gin)                           │
│  ┌──────────────────────────────────────────────────┐  │
│  │         Metrics Service                          │  │
│  │  ┌──────────────┐  ┌──────────────┐            │  │
│  │  │ Aggregator   │  │ Collector    │            │  │
│  │  └──────────────┘  └──────────────┘            │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                          │
                          │ Storage Interface
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Storage Layer                               │
│  ┌──────────────────┐        ┌──────────────────┐      │
│  │ SQLite Adapter   │        │ PostgreSQL Adapter│      │
│  └──────────────────┘        └──────────────────┘      │
└─────────────────────────────────────────────────────────┘
```

## ディレクトリ構成

```
github-activity-metrics/
├── cmd/
│   ├── api/              # API サーバーエントリーポイント
│   └── cli/              # CLI エントリーポイント
├── internal/
│   ├── api/              # API ハンドラー
│   ├── collector/        # GitHub API データ収集
│   ├── aggregator/       # データ集計ロジック
│   ├── domain/           # ドメインモデル
│   ├── storage/          # ストレージ抽象化
│   │   ├── sqlite/
│   │   └── postgres/
│   ├── config/           # 設定管理
│   └── errors/           # エラー定義
├── pkg/
│   └── client/           # API クライアントライブラリ
├── docs/                 # ドキュメント
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

## コントリビューション

1. このリポジトリをフォーク
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add some amazing feature'`)
4. ブランチにプッシュ (`git push origin feature/amazing-feature`)
5. Pull Request を作成

## ライセンス

MIT License - 詳細は [LICENSE](LICENSE) ファイルを参照してください。

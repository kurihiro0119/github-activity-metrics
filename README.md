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

- Go 1.21 以上
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

| 変数名 | 説明 | デフォルト値 |
|--------|------|-------------|
| `GITHUB_TOKEN` | GitHub Personal Access Token | (必須) |
| `STORAGE_TYPE` | ストレージタイプ (`sqlite` または `postgres`) | `sqlite` |
| `SQLITE_PATH` | SQLite データベースファイルのパス | `./metrics.db` |
| `POSTGRES_URL` | PostgreSQL 接続 URL | - |
| `API_PORT` | API サーバーのポート | `8080` |
| `API_HOST` | API サーバーのホスト | `localhost` |
| `API_ENDPOINT` | CLI が使用する API エンドポイント | `http://localhost:8080` |

## 使い方

### CLI

#### データ収集

```bash
# Organization のデータを収集
./bin/github-metrics collect <org-name>

# 期間を指定して収集
./bin/github-metrics collect <org-name> --start 2024-01-01 --end 2024-12-31
```

#### メトリクス表示

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

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/health` | ヘルスチェック |
| GET | `/api/v1/orgs/:org/metrics` | Organization メトリクス |
| GET | `/api/v1/orgs/:org/metrics/timeseries` | 時系列メトリクス |
| GET | `/api/v1/orgs/:org/members/metrics` | 全メンバーメトリクス |
| GET | `/api/v1/orgs/:org/members/:member/metrics` | 特定メンバーメトリクス |
| GET | `/api/v1/orgs/:org/repos/metrics` | 全リポジトリメトリクス |
| GET | `/api/v1/orgs/:org/repos/:repo/metrics` | 特定リポジトリメトリクス |

#### クエリパラメータ

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `start` | 開始日 (YYYY-MM-DD) | 30日前 |
| `end` | 終了日 (YYYY-MM-DD) | 今日 |
| `granularity` | 集計粒度 (day, week, month) | day |
| `type` | メトリクスタイプ (commit, pull_request, deploy) | commit |

#### レスポンス例

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

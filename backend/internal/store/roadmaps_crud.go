package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// このファイルは自分の personal ロードマップの読み出し・更新・削除（SPEC.md §6）。
//
// **所有者の絞り込みは必ず SQL の WHERE に入れる。** 「id で引いてから Go 側で owner を比べる」
// 形にすると、比較を書き忘れた経路が1つあるだけで他人のロードマップが見える。
// 他人のものは「存在しない」（ErrNotFound）として扱い、403 で存在を教えない。

// RoadmapSummary は一覧と更新の応答に使う、ロードマップの見出し部分。
type RoadmapSummary struct {
	ID          string
	Kind        string
	Name        string
	Description string
	Origin      string
	Source      string
	// CheckedAt / TargetDate は未設定なら nil。
	CheckedAt  *time.Time
	TargetDate *time.Time
	ItemCount  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Roadmap は見出しに分野・項目・レベル定義を加えた本体（GET /api/roadmaps/:id）。
type Roadmap struct {
	RoadmapSummary
	Levels  []roadmap.LevelDef
	Domains []Domain
}

// Domain は分野1つ。Items は order_index 順。
type Domain struct {
	ID    string
	Key   string
	Name  string
	Goal  string
	Items []Item
}

// Item は詳細項目1つ。
type Item struct {
	ID          string
	Key         string
	Name        string
	Description string
	Outcome     string
	// OutcomeSource は 'authored' | 'ai_draft'。Outcome が空なら空。
	OutcomeSource string
	VerifyBy      string
	// DependsOn は先に着手すべき項目の key。無ければ空スライス（nil ではない）。
	DependsOn []string
}

// RoadmapUpdate は PATCH /api/roadmaps/:id で変えられる項目。
//
// 「変えない」と「空にする」を区別するため、TargetDate は SetTargetDate と組で使う。
//   - SetTargetDate=false: target_date に触らない
//   - SetTargetDate=true, TargetDate=nil: NULL にする（目標日を消す）
//   - SetTargetDate=true, TargetDate=非nil: その日付にする
type RoadmapUpdate struct {
	// Name は nil なら変更しない。空文字は呼び出し側で弾いておくこと（DB の CHECK で落ちる）
	Name          *string
	TargetDate    *time.Time
	SetTargetDate bool
}

// summaryColumns は RoadmapSummary を読む SELECT 句。r は roadmaps の別名。
// 項目数は毎回サブクエリで数える。一覧は自分の分だけなので件数は小さい
const summaryColumns = `
r.id, r.kind, r.name, coalesce(r.description, ''), r.origin, coalesce(r.source, ''),
r.checked_at, r.target_date,
(SELECT count(*) FROM items i WHERE i.roadmap_id = r.id),
r.created_at, r.updated_at`

// scanner は *sql.Row と *sql.Rows に共通する Scan の口。
type scanner interface {
	Scan(dest ...any) error
}

// scanSummary は summaryColumns の順で1行を読む。
func scanSummary(sc scanner) (*RoadmapSummary, error) {
	var s RoadmapSummary
	var checkedAt, targetDate sql.NullTime
	err := sc.Scan(
		&s.ID, &s.Kind, &s.Name, &s.Description, &s.Origin, &s.Source,
		&checkedAt, &targetDate, &s.ItemCount, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.CheckedAt = nullTimePtr(checkedAt)
	s.TargetDate = nullTimePtr(targetDate)
	return &s, nil
}

// ListRoadmaps は自分の personal ロードマップを、最近更新したものから順に返す。
// 1件も無ければ空スライス（nil ではない）。JSON にしたとき null にならないように。
func (s *Store) ListRoadmaps(ctx context.Context, ownerUserID string) ([]RoadmapSummary, error) {
	q := `SELECT ` + summaryColumns + `
  FROM roadmaps r
 WHERE r.owner_user_id = $1 AND r.kind = $2
 ORDER BY r.updated_at DESC, r.id`

	rows, err := s.db.QueryContext(ctx, q, ownerUserID, RoadmapKindPersonal)
	if err != nil {
		return nil, fmt.Errorf("ロードマップ一覧の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	out := []RoadmapSummary{}
	for rows.Next() {
		sum, err := scanSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("ロードマップ一覧の読み出しに失敗しました: %w", err)
		}
		out = append(out, *sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ロードマップ一覧の走査に失敗しました: %w", err)
	}
	return out, nil
}

// GetRoadmap は自分の personal ロードマップを分野・項目ごと返す。
// 他人のもの・存在しないものは ErrNotFound。
func (s *Store) GetRoadmap(ctx context.Context, ownerUserID, roadmapID string) (*Roadmap, error) {
	q := `SELECT ` + summaryColumns + `, r.levels::text
  FROM roadmaps r
 WHERE r.id = $1 AND r.owner_user_id = $2 AND r.kind = $3`

	var rm Roadmap
	var checkedAt, targetDate sql.NullTime
	var levelsJSON string
	err := s.db.QueryRowContext(ctx, q, roadmapID, ownerUserID, RoadmapKindPersonal).Scan(
		&rm.ID, &rm.Kind, &rm.Name, &rm.Description, &rm.Origin, &rm.Source,
		&checkedAt, &targetDate, &rm.ItemCount, &rm.CreatedAt, &rm.UpdatedAt, &levelsJSON,
	)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("ロードマップの取得に失敗しました: %w", err)
	}
	rm.CheckedAt = nullTimePtr(checkedAt)
	rm.TargetDate = nullTimePtr(targetDate)

	if err := json.Unmarshal([]byte(levelsJSON), &rm.Levels); err != nil {
		// DB の CHECK は「5件の配列」までしか見ていない。壊れていたら握りつぶさず返す
		return nil, fmt.Errorf("レベル定義（levels）の読み出しに失敗しました: %w", err)
	}

	domains, err := s.loadDomains(ctx, roadmapID)
	if err != nil {
		return nil, err
	}
	rm.Domains = domains
	return &rm, nil
}

// loadDomains は分野と項目を order_index 順に読み、分野ごとに項目をぶら下げて返す。
func (s *Store) loadDomains(ctx context.Context, roadmapID string) ([]Domain, error) {
	const domainsQ = `
SELECT id, key, name, coalesce(goal, '')
  FROM domains
 WHERE roadmap_id = $1
 ORDER BY order_index, id`

	rows, err := s.db.QueryContext(ctx, domainsQ, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("分野の取得に失敗しました: %w", err)
	}
	defer rows.Close()

	domains := []Domain{}
	index := map[string]int{} // domain_id -> domains の添字
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Key, &d.Name, &d.Goal); err != nil {
			return nil, fmt.Errorf("分野の読み出しに失敗しました: %w", err)
		}
		d.Items = []Item{}
		index[d.ID] = len(domains)
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("分野の走査に失敗しました: %w", err)
	}

	const itemsQ = `
SELECT domain_id, id, key, name, coalesce(description, ''), coalesce(outcome, ''),
       coalesce(outcome_source, ''), coalesce(verify_by, ''), depends_on_keys
  FROM items
 WHERE roadmap_id = $1
 ORDER BY order_index, id`

	irows, err := s.db.QueryContext(ctx, itemsQ, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("項目の取得に失敗しました: %w", err)
	}
	defer irows.Close()

	for irows.Next() {
		var it Item
		var domainID string
		err := irows.Scan(&domainID, &it.ID, &it.Key, &it.Name, &it.Description, &it.Outcome,
			&it.OutcomeSource, &it.VerifyBy, textArrayScanner(&it.DependsOn))
		if err != nil {
			return nil, fmt.Errorf("項目の読み出しに失敗しました: %w", err)
		}
		if it.DependsOn == nil {
			it.DependsOn = []string{}
		}
		di, ok := index[domainID]
		if !ok {
			// domains.id への FK があるので起きないはず。起きたらデータ不整合として返す
			return nil, fmt.Errorf("項目 %q の分野 %s が見つかりません", it.Key, domainID)
		}
		domains[di].Items = append(domains[di].Items, it)
	}
	if err := irows.Err(); err != nil {
		return nil, fmt.Errorf("項目の走査に失敗しました: %w", err)
	}
	return domains, nil
}

// UpdateRoadmap は名前・目標日を更新し、更新後の見出しを返す。
// 他人のもの・存在しないものは ErrNotFound。updated_at は DB のトリガが進める。
func (s *Store) UpdateRoadmap(ctx context.Context, ownerUserID, roadmapID string, upd RoadmapUpdate) (*RoadmapSummary, error) {
	// 「触らない」は元の値を保つ式で表す。名前は COALESCE、目標日は NULL にする場合があるので CASE
	q := `UPDATE roadmaps r
   SET name        = coalesce($4, r.name),
       target_date = CASE WHEN $5 THEN $6 ELSE r.target_date END
 WHERE r.id = $1 AND r.owner_user_id = $2 AND r.kind = $3
RETURNING ` + summaryColumns

	var targetDate any
	if upd.TargetDate != nil {
		targetDate = *upd.TargetDate
	}
	sum, err := scanSummary(s.db.QueryRowContext(ctx, q,
		roadmapID, ownerUserID, RoadmapKindPersonal, upd.Name, upd.SetTargetDate, targetDate))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("ロードマップの更新に失敗しました: %w", err)
	}
	return sum, nil
}

// DeleteRoadmap は自分の personal ロードマップを削除する。分野・項目・学習ログは FK の cascade で消える。
// 他人のもの・存在しないものは ErrNotFound。
func (s *Store) DeleteRoadmap(ctx context.Context, ownerUserID, roadmapID string) error {
	const q = `DELETE FROM roadmaps WHERE id = $1 AND owner_user_id = $2 AND kind = $3`

	res, err := s.db.ExecContext(ctx, q, roadmapID, ownerUserID, RoadmapKindPersonal)
	if err != nil {
		return fmt.Errorf("ロードマップの削除に失敗しました: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ロードマップの削除件数の取得に失敗しました: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// textArrayScanner は text[] の列を []string に読むための Scanner を返す。
// database/sql は配列を知らないので、pgx の型マップに変換を任せる（KNOWLEDGE.md 2026-08-22）。
func textArrayScanner(dst *[]string) any {
	return pgtype.NewMap().SQLScanner(dst)
}

// nullTimePtr は NULL 許容の時刻を *time.Time に変える。NULL なら nil。
func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

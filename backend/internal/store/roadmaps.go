package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// ロードマップの種類（SPEC.md §3.1 の roadmaps.kind）。
const (
	// RoadmapKindTemplate は公開されて star / fork される側。進捗は紐づかない。
	RoadmapKindTemplate = "template"
	// RoadmapKindPersonal は自分用。学習ログと理解度が紐づく。
	RoadmapKindPersonal = "personal"
)

// ImportRoadmap は検査済みのマスタ JSON から roadmaps / domains / items を作り、roadmaps.id を返す。
//
// 3テーブルへの書き込みは1トランザクションで行う。途中で失敗したときに
// roadmaps と domains だけ残って items が無い、という中途半端な木を作らないため。
//
// **検査（roadmap.Validate）を通った Document を渡すこと。** ここでは内容の妥当性を見ない。
// 重複 key や存在しない依存先がそのまま来ると DB の制約違反として失敗する。
//
// 保存の仕方で決めたこと：
//   - levels は jsonb に丸ごと入れる（SPEC.md §3.2）。LevelDef の JSON タグがそのまま保存形式
//   - depends_on_keys は key の文字列のまま入れ、items.id には解決しない。
//     fork（行の複製）のときに id の張り替えが要らない
//   - outcome が書かれていれば outcome_source は 'authored'（人が書いたもの）。
//     AI 下書き（'ai_draft'）は SPEC.md §4.8 の別経路で入る
//   - visibility は常に 'private'。公開は v2 の publish で別途行う
func (s *Store) ImportRoadmap(ctx context.Context, ownerUserID, kind string, doc *roadmap.Document) (roadmapID string, err error) {
	if doc == nil {
		return "", errors.New("ロードマップの内容が空です")
	}
	if kind != RoadmapKindTemplate && kind != RoadmapKindPersonal {
		return "", fmt.Errorf("ロードマップの種類 %q は不正です", kind)
	}

	// jsonb に入れる形に直す。検査済みなので失敗しないはずだが、握りつぶさない
	levels, err := json.Marshal(doc.Levels)
	if err != nil {
		return "", fmt.Errorf("レベル定義の JSON 変換に失敗しました: %w", err)
	}
	checkedAt, err := parseOptionalDate(doc.CheckedAt)
	if err != nil {
		return "", fmt.Errorf("確認日（checkedAt）が不正です: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("トランザクションの開始に失敗しました: %w", err)
	}
	// Commit 済みなら Rollback は ErrTxDone を返すだけで無害。
	// それ以外の失敗は、元のエラーに添えて返す（握りつぶさない）
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("ロールバックに失敗しました: %w", rbErr))
		}
	}()

	roadmapID, err = insertRoadmap(ctx, tx, ownerUserID, kind, doc, string(levels), checkedAt)
	if err != nil {
		return "", err
	}

	for di, dom := range doc.Domains {
		domainID, err := insertDomain(ctx, tx, roadmapID, dom, di)
		if err != nil {
			return "", err
		}
		for ii, it := range dom.Items {
			if err := insertItem(ctx, tx, roadmapID, domainID, it, ii); err != nil {
				return "", err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("ロードマップの保存の確定に失敗しました: %w", err)
	}
	return roadmapID, nil
}

// insertRoadmap は roadmaps に1行入れて id を返す。
func insertRoadmap(ctx context.Context, tx *sql.Tx, ownerUserID, kind string, doc *roadmap.Document, levels string, checkedAt any) (string, error) {
	const q = `
INSERT INTO roadmaps (kind, owner_user_id, name, description, origin, source, checked_at, levels, visibility)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'private')
RETURNING id`

	var id string
	err := tx.QueryRowContext(ctx, q,
		kind, ownerUserID, doc.Name, nullIfEmpty(doc.Description),
		string(doc.EffectiveOrigin()), nullIfEmpty(doc.Source), checkedAt, levels,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("ロードマップの保存に失敗しました: %w", err)
	}
	return id, nil
}

// insertDomain は domains に1行入れて id を返す。orderIndex は JSON に書かれた順。
func insertDomain(ctx context.Context, tx *sql.Tx, roadmapID string, dom roadmap.DomainDef, orderIndex int) (string, error) {
	const q = `
INSERT INTO domains (roadmap_id, key, name, goal, order_index)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`

	var id string
	err := tx.QueryRowContext(ctx, q, roadmapID, dom.Key, dom.Name, nullIfEmpty(dom.Goal), orderIndex).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("分野 %q の保存に失敗しました: %w", dom.Key, err)
	}
	return id, nil
}

// insertItem は items に1行入れる。orderIndex は分野内で JSON に書かれた順。
func insertItem(ctx context.Context, tx *sql.Tx, roadmapID, domainID string, it roadmap.ItemDef, orderIndex int) error {
	const q = `
INSERT INTO items (roadmap_id, domain_id, key, name, description, outcome, outcome_source, verify_by, depends_on_keys, order_index)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	// 到達状態があるなら、人が書いたものとして記録する（items_outcome_needs_source 制約）
	var outcomeSource any
	if it.Outcome != "" {
		outcomeSource = "authored"
	}
	// depends_on_keys は NOT NULL。nil のまま渡すと NULL になるので空配列に寄せる
	deps := it.DependsOn
	if deps == nil {
		deps = []string{}
	}

	_, err := tx.ExecContext(ctx, q,
		roadmapID, domainID, it.Key, it.Name, nullIfEmpty(it.Description),
		nullIfEmpty(it.Outcome), outcomeSource, nullIfEmpty(it.VerifyBy), deps, orderIndex,
	)
	if err != nil {
		return fmt.Errorf("項目 %q の保存に失敗しました: %w", it.Key, err)
	}
	return nil
}

// parseOptionalDate は YYYY-MM-DD の文字列を date 列に入れる値に変える。
// 空なら NULL（未設定）として扱う。
func parseOptionalDate(s string) (any, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, err
	}
	return t, nil
}

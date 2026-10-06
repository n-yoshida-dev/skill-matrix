package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// このファイルは data/state.json（SPEC.md §3.4）の形と、その書き出し・読み込みを持つ。
//
// **同じ入力から常に同じバイト列になるように書く。** CI の verify はコミットされた state.json と
// 再計算結果をバイト単位で比べる（SPEC.md §3.2・§6）。そのため次を守る。
//   - 生成日時のような「今」に依存する値を入れない
//   - map を使わない（Go の map は反復順が毎回変わる）。項目はロードマップの定義順、履歴は適用順
//   - 空の配列は null ではなく [] で書く（画面の型が配列を前提にしている）

// stateSchemaVersion は state.json の形の版（SPEC.md §3.4）。画面が読めなくなる変え方をしたら上げる（2026-10-06 の retracted のように欄を足すだけなら上げない）。
const stateSchemaVersion = 2

// stateDoc は state.json 全体。
type stateDoc struct {
	SchemaVersion int          `json:"schemaVersion"`
	Items         []itemDoc    `json:"items"`
	Deferred      []pendingDoc `json:"deferred"`
	Rejected      []pendingDoc `json:"rejected"`
	// Retracted は取り消された判定。適用していない。取り消した事実を黙って消さないために残す（SPEC.md §3.4）。
	Retracted []retractedDoc `json:"retracted"`
}

// itemDoc は項目1つの理解度。
type itemDoc struct {
	ItemKey         string `json:"itemKey"`
	VerifiedLevel   int    `json:"verifiedLevel"`
	EvidencedLevels []int  `json:"evidencedLevels"`
	PreState        string `json:"preState"`
	NeedsReview     bool   `json:"needsReview"`
	// LastEvidenceAt は根拠がまだ無ければ null。
	LastEvidenceAt *string    `json:"lastEvidenceAt"`
	Events         []eventDoc `json:"events"`
}

// eventDoc は項目に適用した判定1件の記録。
type eventDoc struct {
	File          string   `json:"file"`
	Index         int      `json:"index"`
	OccurredAt    string   `json:"occurredAt"`
	Source        string   `json:"source"`
	EvidenceType  string   `json:"evidenceType"`
	ProposedLevel int      `json:"proposedLevel"`
	Marked        []int    `json:"marked"`
	EvidenceRefs  []string `json:"evidenceRefs"`
	Rationale     string   `json:"rationale"`
	// Confidence は source が ai 以外なら null。
	Confidence *float64       `json:"confidence"`
	Violations []violationDoc `json:"violations"`
}

// violationDoc は適用した判定に記録された違反（V5 の切り詰め・V6 の不合格の報告）。
type violationDoc struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// pendingDoc は適用しなかった判定（deferred = 保留、rejected = 棄却）1件。
type pendingDoc struct {
	File    string `json:"file"`
	Index   int    `json:"index"`
	ItemKey string `json:"itemKey"`
	Code    string `json:"code"`
	Detail  string `json:"detail"`
}

// retractedDoc は取り消しの記録で適用から外した判定1件。
type retractedDoc struct {
	// File と Index は取り消された判定の場所。
	File  string `json:"file"`
	Index int    `json:"index"`
	// ItemKey は取り消された判定の項目。形の崩れた判定で読めなければ ""。
	ItemKey string `json:"itemKey"`
	// RetractedBy は取り消しを書いたファイル名。
	RetractedBy string `json:"retractedBy"`
	Reason      string `json:"reason"`
}

// encodeState は state.json のバイト列を作る。
//
// 整形は 2 スペースの字下げ・配列も 1 要素 1 行（Go 標準の整形）・末尾に改行 1 つ（SPEC.md §3.4）。
// `<` `>` `&` を < 等に置き換えない。rationale に `List<String>` のような技術用語が入ったとき、
// git diff で読めなくなるため（state.json は HTML に埋め込まない）。
func encodeState(doc stateDoc) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("state.json を組み立てられません: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeState は state.json を読む。recalc の「変わった項目」の表示に、書き直す前の状態を読むために使う。
func decodeState(raw []byte) (stateDoc, error) {
	var doc stateDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return stateDoc{}, fmt.Errorf("state.json を読めません: %w", err)
	}
	return doc, nil
}

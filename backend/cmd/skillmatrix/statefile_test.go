package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/judgment"
)

// このテストは state.json の書き出しが「同じ入力から常に同じバイト列」になることを固定する（SPEC.md §3.2・§3.4）。
// CI の verify はこの性質に頼ってバイト単位で比べる。

func TestEncodeState_同じ入力なら同じバイト列(t *testing.T) {
	a := mustFile(t, "2026-08-01-a.json", `{"schemaVersion": 2, "loggedAt": "2026-08-01", "source": "ai", "judgments": [
		{"itemKey": "go-01", "evidenceType": "self_explanation", "proposedLevel": 2, "evidenceRefs": ["log:a.md"], "rationale": "x", "confidence": 0.85},
		{"itemKey": "react-01", "evidenceType": "implementation", "proposedLevel": 3, "evidenceRefs": ["log:a.md"], "rationale": "y", "confidence": 0.7}]}`)
	b := mustFile(t, "2026-08-02-b.json", `{"schemaVersion": 2, "loggedAt": "2026-08-02", "source": "manual", "judgments": [
		{"itemKey": "go-02", "evidenceType": "explained_to", "proposedLevel": 0, "evidenceRefs": ["log:b.md"], "rationale": "z"}]}`)

	// ファイルの渡し方の順が違っても、何度書き出しても同じ
	first, err := encodeState(recalculate(testRoadmap(), []judgment.File{a, b}, domain.DefaultRules()).doc)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := encodeState(recalculate(testRoadmap(), []judgment.File{b, a}, domain.DefaultRules()).doc)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("%d 回目で違うバイト列になった（最初の違いは %d 行目）", i+1, firstDiffLine(first, again))
		}
	}
}

func TestEncodeState_書き出しの形(t *testing.T) {
	f := mustFile(t, "2026-08-01-a.json", `{"schemaVersion": 2, "loggedAt": "2026-08-01", "source": "manual", "judgments": [
		{"itemKey": "go-01", "evidenceType": "drill", "proposedLevel": 1, "evidenceRefs": ["log:a.md"], "rationale": "List<String> と Map<K, V> & 配列の違いを答えた"}]}`)
	raw, err := encodeState(recalculate(testRoadmap(), []judgment.File{f}, domain.DefaultRules()).doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)

	checks := []struct {
		name string
		want string
	}{
		{"`<` `>` `&` を \\u003c 等に化けさせない", `"rationale": "List<String> と Map<K, V> & 配列の違いを答えた"`},
		{"先頭は schemaVersion で 2 スペースの字下げ", "{\n  \"schemaVersion\": 2,\n  \"items\": ["},
		{"空の配列は null ではなく []", `"deferred": []`},
		{"根拠がまだ無い項目の lastEvidenceAt は null", `"lastEvidenceAt": null`},
		{"manual の confidence は null", `"confidence": null`},
	}
	for _, c := range checks {
		if !strings.Contains(s, c.want) {
			t.Errorf("%s: %q が無い\n%s", c.name, c.want, s)
		}
	}
	if !strings.HasSuffix(s, "}\n") || strings.HasSuffix(s, "\n\n") {
		t.Errorf("末尾は改行 1 つ: %q", s[len(s)-5:])
	}
	for _, bad := range []string{`"events": null`, `"evidencedLevels": null`, `"marked": null`, `"violations": null`} {
		if strings.Contains(s, bad) {
			t.Errorf("%s になっている（画面は配列を前提にしている）", bad)
		}
	}
}

func TestFirstDiffLine(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"a\nb\nc\n", "a\nb\nX\n", 3},
		{"a\n", "a\nb\n", 2},
		{"x", "y", 1},
	}
	for _, tt := range tests {
		if got := firstDiffLine([]byte(tt.a), []byte(tt.b)); got != tt.want {
			t.Errorf("firstDiffLine(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

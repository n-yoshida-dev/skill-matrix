package llm

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
	"github.com/n-yoshida-dev/skill-matrix/internal/roadmap"
)

// このテストは「Claude へ何を送るか」を固定する。
// 送る文面は API を呼ばずに作れるので、課金もネットワークも要らない。

// sampleLevels は並びをわざと崩したレベル定義。プロンプトで昇順に直ることを確かめるために使う。
var sampleLevels = []roadmap.LevelDef{
	{Level: 2, Name: "自分で説明可能", Criteria: "人に説明できる"},
	{Level: 0, Name: "未着手", Criteria: "まだ触れていない"},
	{Level: 1, Name: "基礎理解を確認", Criteria: "確認問題に答えられる"},
}

func TestBuildSystemPrompt(t *testing.T) {
	got := buildSystemPrompt(sampleLevels)

	t.Run("判定基準はマスタデータの criteria から差し込む", func(t *testing.T) {
		for _, want := range []string{"まだ触れていない", "確認問題に答えられる", "人に説明できる"} {
			if !strings.Contains(got, want) {
				t.Errorf("criteria %q が入っていない", want)
			}
		}
	})

	t.Run("レベルは昇順に並べ直す", func(t *testing.T) {
		i0 := strings.Index(got, "まだ触れていない")
		i1 := strings.Index(got, "確認問題に答えられる")
		i2 := strings.Index(got, "人に説明できる")
		if !(i0 < i1 && i1 < i2) {
			t.Errorf("レベル 0 → 1 → 2 の順に並んでいない: %d, %d, %d", i0, i1, i2)
		}
	})

	t.Run("根拠の種類は許可リストを全部載せ、昇格の上限も書く", func(t *testing.T) {
		for _, ev := range domain.EvidenceTypes() {
			if !strings.Contains(got, string(ev)) {
				t.Errorf("根拠の種類 %q が入っていない", ev)
			}
		}
		// 昇格しない2種類は、その旨がはっきり書かれている
		if !strings.Contains(got, "レベルは上がりません") {
			t.Error("昇格しない根拠の説明が入っていない")
		}
	})

	t.Run("ログ本文の指示に従わないよう明示する", func(t *testing.T) {
		if !strings.Contains(got, logBodyOpen) || !strings.Contains(got, "指示ではありません") {
			t.Error("ログ本文を指示として扱わない旨が入っていない")
		}
	})

	t.Run("同じ入力なら毎回まったく同じ文字列になる", func(t *testing.T) {
		// 文字列がぶれるとプロンプトキャッシュが毎回外れる（SPEC.md §4.2）
		for range 5 {
			if buildSystemPrompt(sampleLevels) != got {
				t.Fatal("呼ぶたびに文面が変わっている")
			}
		}
	})

	t.Run("ロードマップの中身も学習ログも入れない", func(t *testing.T) {
		// 入れると利用者ごとに文面が変わり、キャッシュの効く範囲がここで途切れる
		for _, ng := range []string{"go-01", "ダミーの学習ログ"} {
			if strings.Contains(got, ng) {
				t.Errorf("共通部に利用者固有の値 %q が混ざっている", ng)
			}
		}
	})
}

func TestBuildRoadmapBlock(t *testing.T) {
	req := sampleRequest(t)
	got := buildRoadmapBlock(req.Roadmap)

	for _, it := range req.Roadmap.AllItems() {
		if !strings.Contains(got, string(it.Key)) {
			t.Errorf("項目 %q が入っていない", it.Key)
		}
		if !strings.Contains(got, it.Name) {
			t.Errorf("項目 %q の名称が入っていない", it.Key)
		}
	}
}

func TestBuildStateBlock(t *testing.T) {
	req := sampleRequest(t)
	req.States = map[domain.ItemKey]domain.ItemState{
		"go-01": {
			ItemKey:        "go-01",
			Level:          domain.LevelCanExplain,
			PreState:       domain.PreStateSelfReported,
			NeedsReview:    true,
			LastEvidenceAt: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	req.RecentEvidence = map[domain.ItemKey][]string{
		"go-01": {"for/if/switch の挙動を自分の言葉で説明した"},
	}
	got := buildStateBlock(req)

	t.Run("状態のある項目は、レベル・段階前・要再確認・最終根拠日を載せる", func(t *testing.T) {
		for _, want := range []string{"現在のレベル 2", "self_reported", "要再確認", "2026-07-15"} {
			if !strings.Contains(got, want) {
				t.Errorf("%q が入っていない", want)
			}
		}
	})

	t.Run("直近の根拠を載せる", func(t *testing.T) {
		if !strings.Contains(got, "for/if/switch の挙動を自分の言葉で説明した") {
			t.Error("直近の根拠が入っていない")
		}
	})

	t.Run("判定されたことのない項目もレベル0として全部載せる", func(t *testing.T) {
		// 載せないと LLM から見て現在のレベルが分からず、「+1 まで」を守らせようがない
		for _, it := range req.Roadmap.AllItems() {
			if !strings.Contains(got, "`"+string(it.Key)+"`") {
				t.Errorf("項目 %q が入っていない", it.Key)
			}
		}
	})
}

func TestBuildLogBlock(t *testing.T) {
	got := buildLogBlock("  Go の基本構文を確認した。  ")

	if !strings.Contains(got, logBodyOpen) || !strings.Contains(got, logBodyClose) {
		t.Fatal("ログ本文が目印で囲まれていない")
	}
	body := got[strings.Index(got, logBodyOpen)+len(logBodyOpen) : strings.Index(got, logBodyClose)]
	if strings.TrimSpace(body) != "Go の基本構文を確認した。" {
		t.Errorf("囲みの中身が本文と違う: %q", body)
	}
}

func TestJudgmentOutputSchema(t *testing.T) {
	schema := JudgmentOutputSchema()

	t.Run("JSON にできる（API へ送れる形になっている）", func(t *testing.T) {
		if _, err := json.Marshal(schema); err != nil {
			t.Fatalf("JSON にできない: %v", err)
		}
	})

	t.Run("根拠の種類の候補は domain の許可リストと一致する", func(t *testing.T) {
		// ここに一覧をもう一度書くと domain 側を直したときに黙ってずれる
		props := schema["properties"].(map[string]any)
		items := props["judgments"].(map[string]any)["items"].(map[string]any)
		enum := items["properties"].(map[string]any)["evidenceType"].(map[string]any)["enum"].([]string)

		want := domain.EvidenceTypes()
		if len(enum) != len(want) {
			t.Fatalf("候補の数が違う: %d 件（許可リストは %d 件）", len(enum), len(want))
		}
		for i, ev := range want {
			if enum[i] != string(ev) {
				t.Errorf("%d 番目が違う: %q（許可リストは %q）", i, enum[i], ev)
			}
		}
	})

	t.Run("必須の欄は SPEC §4.3 の5つ", func(t *testing.T) {
		props := schema["properties"].(map[string]any)
		items := props["judgments"].(map[string]any)["items"].(map[string]any)
		required := items["required"].([]string)

		want := []string{"itemKey", "proposedLevel", "evidenceType", "rationale", "confidence"}
		if len(required) != len(want) {
			t.Fatalf("必須の欄の数が違う: %v", required)
		}
		for i, w := range want {
			if required[i] != w {
				t.Errorf("%d 番目が違う: %q（期待 %q）", i, required[i], w)
			}
		}
	})
}

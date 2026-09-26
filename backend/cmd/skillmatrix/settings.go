package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/n-yoshida-dev/skill-matrix/internal/domain"
)

// このファイルは data/settings.json（SPEC.md §8.3）を読む。画面と共通の設定で、無ければ既定値を使う。

// settings は CLI が使う設定。
type settings struct {
	rules     domain.Rules
	staleness domain.StalenessConfig
	weights   domain.Weights
	// nextActionsLimit は recalc の要約に出す「次にやること」の件数。
	nextActionsLimit int
}

// defaultSettings は settings.json が無いときの既定値。値の正本は domain の Default*（SPEC.md §8.3）。
func defaultSettings() settings {
	return settings{
		rules:            domain.DefaultRules(),
		staleness:        domain.DefaultStalenessConfig(),
		weights:          domain.DefaultWeights(),
		nextActionsLimit: defaultNextActionsLimit,
	}
}

// defaultNextActionsLimit は「次にやること」の既定件数（SPEC.md §8.3 の nextActions.limit）。
const defaultNextActionsLimit = 5

// wireSettings は settings.json の形。書かれていない欄は既定値のままにするため、ポインタで受ける。
type wireSettings struct {
	Rules *struct {
		ConfidenceThreshold *float64 `json:"confidenceThreshold"`
		MaxItemsPerLog      *int     `json:"maxItemsPerLog"`
	} `json:"rules"`
	Staleness *struct {
		FreshWithinDays *int `json:"freshWithinDays"`
		AgingWithinDays *int `json:"agingWithinDays"`
	} `json:"staleness"`
	Weights *struct {
		Readiness *float64 `json:"readiness"`
		Gap       *float64 `json:"gap"`
		Staleness *float64 `json:"staleness"`
		Unlocks   *float64 `json:"unlocks"`
	} `json:"weights"`
	NextActions *struct {
		Limit *int `json:"limit"`
	} `json:"nextActions"`
	// Site は画面だけが使う（ヘッダ・フッターのリンク先）。CLI は読まないが、未知のキーとして弾かないために定義する。
	Site *struct {
		RepoURL *string `json:"repoUrl"`
	} `json:"site"`
}

// parseSettings は settings.json を読む。書かれていない欄は既定値のまま。
//
// **未知のキーはエラーにする。** `confidenceThreshhold` のような打ち間違いで、閾値が黙って既定値に戻るのを防ぐ。
func parseSettings(raw []byte) (settings, error) {
	var w wireSettings
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return settings{}, fmt.Errorf("settings.json を読めません: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return settings{}, errors.New("settings.json: JSON オブジェクトの後ろに余分なデータがあります")
	}

	s := defaultSettings()
	const day = 24 * time.Hour
	if r := w.Rules; r != nil {
		setIf(&s.rules.ConfidenceThreshold, r.ConfidenceThreshold)
		setIf(&s.rules.MaxItemsPerLog, r.MaxItemsPerLog)
	}
	if st := w.Staleness; st != nil {
		if st.FreshWithinDays != nil {
			s.staleness.FreshWithin = time.Duration(*st.FreshWithinDays) * day
		}
		if st.AgingWithinDays != nil {
			s.staleness.AgingWithin = time.Duration(*st.AgingWithinDays) * day
		}
	}
	if wt := w.Weights; wt != nil {
		setIf(&s.weights.Readiness, wt.Readiness)
		setIf(&s.weights.Gap, wt.Gap)
		setIf(&s.weights.Staleness, wt.Staleness)
		setIf(&s.weights.Unlocks, wt.Unlocks)
	}
	if na := w.NextActions; na != nil {
		setIf(&s.nextActionsLimit, na.Limit)
	}
	if s.nextActionsLimit < 0 {
		return settings{}, fmt.Errorf("settings.json: nextActions.limit は 0 以上にしてください（%d）", s.nextActionsLimit)
	}
	return s, nil
}

// setIf は値が書かれていたときだけ上書きする。
func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

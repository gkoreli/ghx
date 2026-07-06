package evals

// SPTValue represents one signal-per-token value. Defined is false when the
// ADR declares a level undefined for the profile, rather than numerically zero.
type SPTValue struct {
	Defined bool    `json:"defined"`
	Value   float64 `json:"value,omitempty"`
}

// EpisodeSPT is the ADR-0016.6 signal-per-token view for one episode.
type EpisodeSPT struct {
	Profile Profile `json:"profile"`
	Signal  float64 `json:"signal"`

	MainAgentSPT       SPTValue `json:"mainAgentSpt"`
	SidecarInternalSPT SPTValue `json:"sidecarInternalSpt"`
	WorkflowSPT        SPTValue `json:"workflowSpt"`
}

// ProfileSPT is the per-profile aggregate ADR-0016.6 SPT report.
type ProfileSPT struct {
	Profile  Profile `json:"profile"`
	Episodes int     `json:"episodes"`

	MeanSignal float64 `json:"meanSignal"`

	MainAgentSPT       SPTValue `json:"mainAgentSpt"`
	SidecarInternalSPT SPTValue `json:"sidecarInternalSpt"`
	WorkflowSPT        SPTValue `json:"workflowSpt"`
}

// EpisodeSignalPerToken computes ADR-0016.6 SPT for one episode:
// signal = correctness × evidence, tokens = chars/4, scaled per 1,000 tokens.
func EpisodeSignalPerToken(ep *Episode) EpisodeSPT {
	if ep == nil {
		return EpisodeSPT{}
	}
	signal := ep.Rewards.Correctness * ep.Rewards.Evidence
	return EpisodeSPT{
		Profile:            ep.Profile,
		Signal:             signal,
		MainAgentSPT:       sptFromChars(signal, ep.Context.MainAgentChars),
		SidecarInternalSPT: sidecarInternalSPT(ep.Profile, signal, ep.Context.SidecarInternalChars),
		WorkflowSPT:        sptFromChars(signal, ep.Context.TotalWorkflowChars),
	}
}

// AggregateSignalPerToken computes per-profile aggregate SPT over valid
// episodes only, matching EvaluateGates' exclusion semantics.
func AggregateSignalPerToken(episodes []*Episode) map[Profile]ProfileSPT {
	filtered, _, _ := validateEpisodesForVerdict(episodes)

	type sums struct {
		episodes             int
		signal               float64
		mainAgentChars       int
		sidecarInternalChars int
		totalWorkflowChars   int
	}
	bag := map[Profile]*sums{}
	for _, p := range AllProfiles() {
		bag[p] = &sums{}
	}
	for _, ep := range filtered {
		s, ok := bag[ep.Profile]
		if !ok {
			continue
		}
		signal := ep.Rewards.Correctness * ep.Rewards.Evidence
		s.episodes++
		s.signal += signal
		s.mainAgentChars += ep.Context.MainAgentChars
		s.sidecarInternalChars += ep.Context.SidecarInternalChars
		s.totalWorkflowChars += ep.Context.TotalWorkflowChars
	}

	out := map[Profile]ProfileSPT{}
	for _, p := range AllProfiles() {
		s := bag[p]
		row := ProfileSPT{
			Profile:            p,
			Episodes:           s.episodes,
			MainAgentSPT:       sptFromChars(s.signal, s.mainAgentChars),
			SidecarInternalSPT: sidecarInternalSPT(p, s.signal, s.sidecarInternalChars),
			WorkflowSPT:        sptFromChars(s.signal, s.totalWorkflowChars),
		}
		if s.episodes > 0 {
			row.MeanSignal = s.signal / float64(s.episodes)
		}
		out[p] = row
	}
	return out
}

// ProfileRealTokenSPT is the per-profile ADR-0016.11 real-token SPT report:
// signal per 1,000 provider-reported tokens, session-level (one level, not
// three — provider usage cannot be attributed across the main-agent/sidecar
// boundary). Available only when every episode in the aggregate has usage
// on every turn (D2: all-or-UNAVAILABLE, never a silent partial mean).
type ProfileRealTokenSPT struct {
	Profile  Profile `json:"profile"`
	Episodes int     `json:"episodes"`
	// EpisodesWithUsage counts episodes whose every turn carries provider
	// usage. Available requires EpisodesWithUsage == Episodes > 0.
	EpisodesWithUsage int  `json:"episodesWithUsage"`
	Available         bool `json:"available"`

	// TotalTokens sums input + cacheRead + cacheCreation + output tokens
	// over all covered episodes (ADR-0016.11 D1: all-processed-tokens).
	// Zero unless Available.
	TotalTokens int      `json:"totalTokens,omitempty"`
	SessionSPT  SPTValue `json:"sessionSpt"`
}

// episodeRealTokenUsage folds one episode's per-turn provider usage
// (ADR-0016.10 D6 persistence) into the ADR-0016.11 views. hasUsage is true
// iff the episode has at least one turn and every turn carries usage;
// hasCost additionally requires every turn's usage source to be "result"
// (only terminal result messages carry total_cost_usd).
func episodeRealTokenUsage(ep *Episode) (tokens int, costUSD float64, hasUsage, hasCost bool) {
	if ep == nil || len(ep.Turns) == 0 {
		return 0, 0, false, false
	}
	hasUsage, hasCost = true, true
	for _, t := range ep.Turns {
		if t.RawSDK == nil || t.RawSDK.Usage == nil {
			return 0, 0, false, false
		}
		u := t.RawSDK.Usage
		tokens += u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens + u.OutputTokens
		if u.Source != "result" {
			hasCost = false
		}
		costUSD += u.CostUSD
	}
	if !hasCost {
		costUSD = 0
	}
	return tokens, costUSD, true, hasCost
}

// AggregateRealTokenSPT computes the ADR-0016.11 real-token SPT per profile
// over the same gate-filtered episode set as the chars/4 variant, summing
// signal and provider tokens before dividing. It never partial-means: a
// profile with any usage-less episode reports Available=false with counts.
func AggregateRealTokenSPT(episodes []*Episode) map[Profile]ProfileRealTokenSPT {
	filtered, _, _ := validateEpisodesForVerdict(episodes)

	type sums struct {
		episodes  int
		withUsage int
		signal    float64
		tokens    int
	}
	bag := map[Profile]*sums{}
	for _, p := range AllProfiles() {
		bag[p] = &sums{}
	}
	for _, ep := range filtered {
		s, ok := bag[ep.Profile]
		if !ok {
			continue
		}
		s.episodes++
		tokens, _, hasUsage, _ := episodeRealTokenUsage(ep)
		if !hasUsage {
			continue
		}
		s.withUsage++
		s.signal += ep.Rewards.Correctness * ep.Rewards.Evidence
		s.tokens += tokens
	}

	out := map[Profile]ProfileRealTokenSPT{}
	for _, p := range AllProfiles() {
		s := bag[p]
		row := ProfileRealTokenSPT{
			Profile:           p,
			Episodes:          s.episodes,
			EpisodesWithUsage: s.withUsage,
			Available:         s.episodes > 0 && s.withUsage == s.episodes,
		}
		if row.Available {
			row.TotalTokens = s.tokens
			if s.tokens > 0 {
				row.SessionSPT = SPTValue{Defined: true, Value: s.signal / float64(s.tokens) * 1000.0}
			}
		}
		out[p] = row
	}
	return out
}

// ProfileEconomics is one profile's share of the run cost (ADR-0016.11 D4).
type ProfileEconomics struct {
	Profile  Profile `json:"profile"`
	Episodes int     `json:"episodes"`
	// EpisodesWithCost counts episodes whose every turn carries terminal
	// ("result") usage — the only source that prices the turn.
	EpisodesWithCost int     `json:"episodesWithCost"`
	CostUSD          float64 `json:"costUsd"`
}

// RunEconomics is the run-level cost report (ADR-0016.11 D4): costUsd summed
// over ALL loaded episodes — including gate-excluded ones, because excluded
// episodes still spent money (registered asymmetry with SPT, which uses the
// gate-filtered set). Totals are lower bounds when coverage is partial.
type RunEconomics struct {
	Episodes         int                `json:"episodes"`
	EpisodesWithCost int                `json:"episodesWithCost"`
	TotalCostUSD     float64            `json:"totalCostUsd"`
	PerProfile       []ProfileEconomics `json:"perProfile"`
}

// ComputeRunEconomics sums per-episode costUsd over every non-nil episode.
func ComputeRunEconomics(episodes []*Episode) RunEconomics {
	byProfile := map[Profile]*ProfileEconomics{}
	for _, p := range AllProfiles() {
		byProfile[p] = &ProfileEconomics{Profile: p}
	}
	econ := RunEconomics{}
	for _, ep := range episodes {
		if ep == nil {
			continue
		}
		econ.Episodes++
		_, cost, _, hasCost := episodeRealTokenUsage(ep)
		pe := byProfile[ep.Profile]
		if pe != nil {
			pe.Episodes++
		}
		if !hasCost {
			continue
		}
		econ.EpisodesWithCost++
		econ.TotalCostUSD += cost
		if pe != nil {
			pe.EpisodesWithCost++
			pe.CostUSD += cost
		}
	}
	for _, p := range AllProfiles() {
		econ.PerProfile = append(econ.PerProfile, *byProfile[p])
	}
	return econ
}

func sidecarInternalSPT(profile Profile, signal float64, chars int) SPTValue {
	if profile != ProfileSidecar {
		return SPTValue{Defined: false}
	}
	return sptFromChars(signal, chars)
}

func sptFromChars(signal float64, chars int) SPTValue {
	if chars <= 0 {
		return SPTValue{Defined: false}
	}
	return SPTValue{
		Defined: true,
		Value:   signal / (float64(chars) / 4.0) * 1000.0,
	}
}

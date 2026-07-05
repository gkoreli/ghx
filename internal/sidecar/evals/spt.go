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

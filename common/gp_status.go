package common

// GP statuses
// should probably be in gpcm, qr2 needs it though

const (
	Offline = iota
	Online
	Playing
	MatchAnybody
	MatchFriend

	MatchClient
	MatchServer
)

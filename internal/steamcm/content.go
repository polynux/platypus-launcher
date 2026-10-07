// High-level CM calls: token exchange, depot keys, PICS, server list,
// request codes and CDN tokens. Field numbers come from
// SteamDatabase/Protobufs (steammessages_auth.steamclient.proto,
// steammessages_clientserver_2.proto, steammessages_clientserver_appinfo.proto,
// steammessages_contentsystem.steamclient.proto).
package steamcm

import (
	"context"
	"fmt"
	"time"

	"hydra-launcher/internal/depot"
	"hydra-launcher/internal/steam"
)

// Exchange trades the refresh token for an access token over an
// authenticated CM session, like steam_cm.py's exchange(). Renewal is
// deliberately not requested: renewing would invalidate the refresh token
// DepotDownloader keeps in its login key.
func Exchange(ctx context.Context, refreshToken string) (string, error) {
	steamID, err := SteamIDFromToken(refreshToken)
	if err != nil {
		return "", err
	}
	s, err := Dial(ctx, refreshToken)
	if err != nil {
		return "", err
	}
	defer s.Close()
	e := &depot.Enc{}
	e.Str(1, refreshToken)
	e.Fixed64(2, steamID)
	body, err := s.Call("Authentication.GenerateAccessTokenForApp#1", e.Bytes(), 30*time.Second)
	if err != nil {
		return "", err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return "", &CMError{Msg: fmt.Sprintf("Steam's token response didn't parse: %v", err)}
	}
	if tok := m.Bytes(1); len(tok) > 0 {
		return string(tok), nil
	}
	return "", &CMError{Msg: "Steam's token response carried no access token"}
}

// DepotKey fetches one depot's decryption key (EMsg 5438/5439,
// CMsgClientGetDepotDecryptionKey: depot_id = 1, app_id = 2; the response
// carries eresult = 1, depot_id = 2, depot_encryption_key = 3). These are
// plain CM messages, not service calls.
func (s *Session) DepotKey(depotID, appID uint32, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	e := &depot.Enc{}
	e.Varint(1, uint64(depotID))
	e.Varint(2, uint64(appID))
	s.jobID++
	job := s.jobID
	hdr := header(s.steamID, s.sessionID, job, "")
	if err := s.conn.WriteMessage(2, packEMsg(EMsgClientGetDepotDecryptionKey, hdr, e.Bytes())); err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(timeout)
	for {
		emsg, rhdr, body, err := s.readOne(deadline)
		if err != nil {
			return nil, err
		}
		if emsg == EMsgClientHeartBeat {
			s.sideMessage(emsg, rhdr, body)
			continue
		}
		if emsg == EMsgClientGetDepotDecryptionKey+1 && rhdr.Int(11) == job {
			m, err := depot.Decode(body)
			if err != nil {
				return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			if eresult := int(m.Int(1)); eresult != 1 {
				auth := tokenRejected(eresult)
				return nil, &CMError{Msg: "Steam rejected the request with " + EResultName(eresult), EResult: eresult, Auth: auth}
			}
			return append([]byte{}, m.Bytes(3)...), nil
		}
		s.sideMessage(emsg, rhdr, body)
	}
}

// PICSAccessTokens fetches access tokens for apps (and packages), needed
// before product info (EMsg 8905/8906: packageids = 1, appids = 2;
// response: package_access_tokens = 1, app_access_tokens = 3, each with
// packageid/appid = 1 and access_token = 2).
func (s *Session) PICSAccessTokens(appIDs, packageIDs []uint32, timeout time.Duration) (map[uint32]uint64, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	e := &depot.Enc{}
	for _, p := range packageIDs {
		e.Varint(1, uint64(p))
	}
	for _, a := range appIDs {
		e.Varint(2, uint64(a))
	}
	s.jobID++
	job := s.jobID
	if err := s.conn.WriteMessage(2, packEMsg(EMsgClientPICSAccessToken, header(s.steamID, s.sessionID, job, ""), e.Bytes())); err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(timeout)
	for {
		emsg, rhdr, body, err := s.readOne(deadline)
		if err != nil {
			return nil, err
		}
		if emsg == EMsgClientHeartBeat {
			s.sideMessage(emsg, rhdr, body)
			continue
		}
		if emsg == EMsgClientPICSAccessToken+1 && rhdr.Int(11) == job {
			m, err := depot.Decode(body)
			if err != nil {
				return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			out := map[uint32]uint64{}
			for _, t := range m.AllBytes(3) {
				if tm, err := depot.Decode(t); err == nil {
					out[uint32(tm.Int(1))] = tm.Int(2)
				}
			}
			return out, nil
		}
		s.sideMessage(emsg, rhdr, body)
	}
}

// ProductInfo fetches one app's PICS info (EMsg 8903/8904: apps = 2 with
// appid = 1 and access_token = 2) and parses the buffer with the text
// KeyValues parser. Missing tokens come back flagged; unknown apps are
// an error, not an empty reply.
func (s *Session) ProductInfo(appID uint32, accessToken uint64, timeout time.Duration) (*steam.KV, uint32, error) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	app := &depot.Enc{}
	app.Varint(1, uint64(appID))
	if accessToken != 0 {
		app.Varint(2, accessToken)
	}
	e := &depot.Enc{}
	e.MsgField(2, app)
	s.jobID++
	job := s.jobID
	if err := s.conn.WriteMessage(2, packEMsg(EMsgClientPICSProductInfo, header(s.steamID, s.sessionID, job, ""), e.Bytes())); err != nil {
		return nil, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(timeout)
	for {
		emsg, rhdr, body, err := s.readOne(deadline)
		if err != nil {
			return nil, 0, err
		}
		if emsg == EMsgClientHeartBeat {
			s.sideMessage(emsg, rhdr, body)
			continue
		}
		if emsg == EMsgClientPICSProductInfo+1 && rhdr.Int(11) == job {
			m, err := depot.Decode(body)
			if err != nil {
				return nil, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			if len(m.AllBytes(1)) == 0 {
				return nil, 0, &CMError{Msg: fmt.Sprintf("Steam has no product info for app %d", appID)}
			}
			am, err := depot.Decode(m.AllBytes(1)[0])
			if err != nil {
				return nil, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			if am.Int(3) != 0 {
				return nil, 0, &CMError{Msg: fmt.Sprintf("Steam needs an access token for app %d", appID), Auth: true}
			}
			kv, err := steam.ParseKV(string(am.Bytes(5)))
			if err != nil {
				return nil, 0, fmt.Errorf("product info didn't parse: %w", err)
			}
			return kv, uint32(am.Int(2)), nil
		}
		s.sideMessage(emsg, rhdr, body)
	}
}

// Server describes one CDN host from GetServersForSteamPipe.
type Server struct {
	Type          string
	Host          string
	VHost         string
	UseHTTPS      bool
	AllowedAppIDs []uint32
	WeightedLoad  float32
	CellID        int32
}

// ServersForSteamPipe fetches the download sources (ContentServerDirectory.
// GetServersForSteamPipe#1: cell_id = 1, max_servers = 2). DepotDownloader
// keeps types SteamCache and CDN, honours allowed_app_ids, and sorts by
// penalty then weighted_load; num_entries_in_client_list repeats a server.
func (s *Session) ServersForSteamPipe(cellID uint32, maxServers uint32, timeout time.Duration) ([]Server, error) {
	if maxServers == 0 {
		maxServers = 20
	}
	e := &depot.Enc{}
	e.Varint(1, uint64(cellID))
	e.Varint(2, uint64(maxServers))
	body, err := s.Call("ContentServerDirectory.GetServersForSteamPipe#1", e.Bytes(), timeout)
	if err != nil {
		return nil, err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	var out []Server
	for _, raw := range m.AllBytes(1) {
		sm, err := depot.Decode(raw)
		if err != nil {
			continue
		}
		sv := Server{
			Type:   string(sm.Bytes(1)),
			Host:   string(sm.Bytes(8)),
			VHost:  string(sm.Bytes(9)),
			CellID: int32(sm.Int(3)),
		}
		switch string(sm.Bytes(12)) {
		case "https", "all", "valid":
			sv.UseHTTPS = true
		}
		if f := sm.AllInts(5); len(f) > 0 {
			// weighted_load is a float32 on the wire (fixed32).
			bits := uint32(f[0])
			sv.WeightedLoad = float32FromBits(bits)
		}
		for _, a := range sm.AllInts(13) {
			sv.AllowedAppIDs = append(sv.AllowedAppIDs, uint32(a))
		}
		repeat := int(sm.Int(6))
		if repeat < 1 {
			repeat = 1
		}
		for i := 0; i < repeat; i++ {
			out = append(out, sv)
		}
	}
	return out, nil
}

// ManifestRequestCode fetches a manifest's request code
// (GetManifestRequestCode#1: app_id = 1, depot_id = 2, manifest_id = 3,
// app_branch = 4, branch_password_hash = 5; codes expire after ~5 min).
func (s *Session) ManifestRequestCode(appID, depotID uint32, manifestID uint64, branch, branchPasswordHash string, timeout time.Duration) (uint64, error) {
	e := &depot.Enc{}
	e.Varint(1, uint64(appID))
	e.Varint(2, uint64(depotID))
	e.Varint(3, manifestID)
	if branch != "" {
		e.Str(4, branch)
	}
	if branchPasswordHash != "" {
		e.Str(5, branchPasswordHash)
	}
	body, err := s.Call("ContentServerDirectory.GetManifestRequestCode#1", e.Bytes(), timeout)
	if err != nil {
		return 0, err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	return m.Int(1), nil
}

// CDNAuthToken fetches a host's auth token (GetCDNAuthToken#1: depot_id = 1,
// host_name = 2, app_id = 3; response: token = 1, expiration_time = 2).
func (s *Session) CDNAuthToken(depotID uint32, host string, appID uint32, timeout time.Duration) (string, uint32, error) {
	e := &depot.Enc{}
	e.Varint(1, uint64(depotID))
	e.Str(2, host)
	e.Varint(3, uint64(appID))
	body, err := s.Call("ContentServerDirectory.GetCDNAuthToken#1", e.Bytes(), timeout)
	if err != nil {
		return "", 0, err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return "", 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	return string(m.Bytes(1)), uint32(m.Int(2)), nil
}

// CheckBetaPassword checks a password branch (EMsg 5450/5451:
// CMsgClientCheckAppBetaPassword: app_id = 1, betapassword = 2).
func (s *Session) CheckBetaPassword(appID uint32, password string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	e := &depot.Enc{}
	e.Varint(1, uint64(appID))
	e.Str(2, password)
	s.jobID++
	job := s.jobID
	if err := s.conn.WriteMessage(2, packEMsg(EMsgClientCheckAppBetaPassword, header(s.steamID, s.sessionID, job, ""), e.Bytes())); err != nil {
		return &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(timeout)
	for {
		emsg, rhdr, body, err := s.readOne(deadline)
		if err != nil {
			return err
		}
		if emsg == EMsgClientHeartBeat {
			s.sideMessage(emsg, rhdr, body)
			continue
		}
		if emsg == EMsgClientCheckAppBetaPassword+1 && rhdr.Int(11) == job {
			m, err := depot.Decode(body)
			if err != nil {
				return &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			if eresult := int(m.Int(1)); eresult != 1 {
				return &CMError{Msg: "Steam rejected the request with " + EResultName(eresult), EResult: eresult, Auth: tokenRejected(eresult)}
			}
			return nil
		}
		s.sideMessage(emsg, rhdr, body)
	}
}

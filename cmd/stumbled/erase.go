package main

import "context"

// eraseBaas removes the accounts keyed by these BAAS user ids (the NSA id token's sub claim is the
// BAAS user id) and the kept analytics bodies that name their Stumble id. Card openpak/prds#120.
// Accounts that logged in without an NSA token ("device:<id>") are keyed by a device id the core never
// sees, so they cannot be tied to an OpenPak account and are not touched.
func (s *store) eraseBaas(ctx context.Context, baas []string) (n int, ids []int, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	rows, err := tx.Query(ctx, `SELECT id, stumble_id FROM stumble_accounts WHERE key = ANY($1)`, baas)
	if err != nil {
		return 0, nil, err
	}
	var sids []string
	for rows.Next() {
		var id int
		var sid string
		if err := rows.Scan(&id, &sid); err != nil {
			rows.Close()
			return 0, nil, err
		}
		ids = append(ids, id)
		sids = append(sids, sid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	for _, sid := range sids {
		t, err := tx.Exec(ctx, `DELETE FROM stumble_collector WHERE position(convert_to($1, 'UTF8') in body) > 0`, sid)
		if err != nil {
			return 0, nil, err
		}
		n += int(t.RowsAffected())
	}
	t, err := tx.Exec(ctx, `DELETE FROM stumble_accounts WHERE key = ANY($1)`, baas)
	if err != nil {
		return 0, nil, err
	}
	n += int(t.RowsAffected())
	return n, ids, tx.Commit(ctx)
}

// erase resolves the account's BAAS ids through the core and erases them.
func (s *server) erase(ctx context.Context, account string) (int, error) {
	if s.links == nil {
		return 0, errNoCore
	}
	baas, err := s.links.SwitchSubjects(ctx, account)
	if err != nil {
		return 0, err
	}
	if len(baas) == 0 {
		return 0, nil
	}
	n, ids, err := s.users.eraseBaas(ctx, baas)
	if err != nil {
		return 0, err
	}
	// The matchmaker's in-memory tickets and queued requests carry the numeric user id.
	gone := map[int]bool{}
	for _, id := range ids {
		gone[id] = true
	}
	s.mm.mu.Lock()
	for k, p := range s.mm.tickets {
		if gone[p.UserId] {
			delete(s.mm.tickets, k)
		}
	}
	for k, r := range s.mm.requests {
		if gone[r.p.UserId] {
			delete(s.mm.requests, k)
		}
	}
	s.mm.mu.Unlock()
	return n, nil
}

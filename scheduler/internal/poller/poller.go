package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/publisher"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/pubmed"
	"github.com/nysthretha/pubmed_literature_alert_service/scheduler/internal/store"
)

const (
	pageSize         = 200
	firstRunLookback = 30 * 24 * time.Hour
)

type Poller struct {
	store     *store.Store
	client    *pubmed.Client
	publisher *publisher.Publisher
}

func New(s *store.Store, c *pubmed.Client, p *publisher.Publisher) *Poller {
	return &Poller{store: s, client: c, publisher: p}
}

func (p *Poller) RunOnce(ctx context.Context) error {
	queries, err := p.store.Queries(ctx)
	if err != nil {
		return err
	}
	for _, q := range queries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := p.pollOne(ctx, q); err != nil {
			slog.Error("poll query failed", "query_id", q.ID, "name", q.Name, "err", err.Error())
		}
	}
	return nil
}

func (p *Poller) pollOne(ctx context.Context, q store.Query) error {
	start := time.Now().UTC()

	var mindate string
	if q.LastPolledAt != nil {
		mindate = q.LastPolledAt.Format("2006/01/02")
	} else {
		mindate = start.Add(-firstRunLookback).Format("2006/01/02")
	}
	maxdate := start.Format("2006/01/02")

	slog.Info("poll start", "query_id", q.ID, "name", q.Name, "mindate", mindate, "maxdate", maxdate)

	var total, published int
	retstart := 0
	for {
		r, err := p.client.Search(ctx, q.QueryString, mindate, maxdate, pageSize, retstart)
		if err != nil {
			return err
		}
		total = r.Count
		for _, pmid := range r.PMIDs {
			if err := p.publisher.Publish(ctx, publisher.FetchMessage{PMID: pmid, QueryID: q.ID}); err != nil {
				return err
			}
			published++
		}
		if len(r.PMIDs) == 0 || retstart+pageSize >= total {
			break
		}
		retstart += pageSize
	}

	if err := p.store.UpdateLastPolled(ctx, q.ID, start); err != nil {
		return err
	}

	slog.Info("poll end", "query_id", q.ID, "name", q.Name, "total_reported", total, "published", published, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

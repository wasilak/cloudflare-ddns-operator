package controller

import (
	"log/slog"

	"context"

	"github.com/cloudflare/cloudflare-go"
	dnsentriesv1 "github.com/wasilak/cloudflare-ddns-operator/api/v1"
)

func getRecordsFromCRD(ctx context.Context, crdInstance *dnsentriesv1.DnsEntry) ([]cloudflare.DNSRecord, error) {
	records := []cloudflare.DNSRecord{}

	for _, v := range crdInstance.Spec.Items {

		recordTmp := cloudflare.DNSRecord{
			Name:     v.Name,
			Content:  "127.0.0.1",
			Proxied:  &v.Proxied,
			TTL:      v.TTL,
			ZoneName: v.ZoneName,
			Type:     v.Type,
		}

		records = append(records, recordTmp)
	}

	slog.InfoContext(ctx, "records", "records", records)

	return records, nil
}

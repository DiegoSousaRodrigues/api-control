package accountv2

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type readRepositoryStub struct {
	summary *summaryRecord
	exists  bool
	pages   [][]statementRecord
	queries []statementQuery
}

func (stub *readRepositoryStub) Summary(context.Context, int64) (*summaryRecord, error) {
	return stub.summary, nil
}
func (stub *readRepositoryStub) ClientExists(context.Context, int64) (bool, error) {
	return stub.exists, nil
}
func (stub *readRepositoryStub) Statement(_ context.Context, query statementQuery) ([]statementRecord, error) {
	stub.queries = append(stub.queries, query)
	page := stub.pages[0]
	stub.pages = stub.pages[1:]
	return page, nil
}

func TestReadSummaryClassifiesCreditAndCountsOpenInvoices(t *testing.T) {
	stub := &readRepositoryStub{summary: &summaryRecord{ClientID: 1, ClientName: "Client", ClientActive: false,
		NetBalance: decimal.RequireFromString("-100.00"), OpenInvoiceCount: 2}}
	service, err := NewReadService(stub, func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.Summary(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if response.Position != PositionCredit || !response.CreditAmount.Decimal().Equal(decimal.NewFromInt(100)) ||
		response.OpenInvoiceCount != 2 || response.Client.Active {
		t.Fatalf("summary = %#v", response)
	}
}

func TestStatementCursorKeepsCutoffTupleAndDateToStable(t *testing.T) {
	firstNow := time.Date(2026, 8, 11, 15, 0, 0, 0, time.UTC)
	effective := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	recorded := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	stub := &readRepositoryStub{exists: true, pages: [][]statementRecord{
		{
			{EventID: "payment:2", Type: "payment_posted", EffectiveDate: effective, RecordedAt: recorded.Add(time.Hour), PaymentID: int64Pointer(2), Credit: decimal.NewFromInt(50), BalanceAfterEvent: decimal.NewFromInt(50), SourceID: 2},
			{EventID: "invoice:1", Type: "invoice_issued", EffectiveDate: effective, RecordedAt: recorded, InvoiceID: int64Pointer(1), Debit: decimal.NewFromInt(100), BalanceAfterEvent: decimal.NewFromInt(100), SourceID: 1},
		},
		{},
	}}
	nowCalls := 0
	service, err := NewReadService(stub, func() time.Time {
		nowCalls++
		return firstNow.Add(time.Duration(nowCalls-1) * time.Hour)
	})
	if err != nil {
		t.Fatal(err)
	}
	dateTo := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	first, err := service.Statement(context.Background(), 1, StatementFilter{Limit: 1, DateTo: &dateTo})
	if err != nil {
		t.Fatal(err)
	}
	if first.NextCursor == nil || len(first.Items) != 1 || !first.SnapshotRecordedAt.Equal(firstNow) {
		t.Fatalf("first page = %#v", first)
	}
	second, err := service.Statement(context.Background(), 1, StatementFilter{Limit: 1, DateTo: &dateTo, Cursor: *first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if !second.SnapshotRecordedAt.Equal(first.SnapshotRecordedAt) || len(stub.queries) != 2 ||
		!stub.queries[1].Cutoff.Equal(stub.queries[0].Cutoff) || stub.queries[1].Cursor == nil ||
		stub.queries[1].Cursor.EventIDForTest() != "payment_posted:2" {
		t.Fatalf("unstable cursor: first=%#v second=%#v queries=%#v", first, second, stub.queries)
	}
}

func TestStatementCursorIsBoundToClientAndCannotMoveCutoffForward(t *testing.T) {
	now := time.Date(2026, 8, 11, 15, 0, 0, 0, time.UTC)
	stub := &readRepositoryStub{exists: true}
	service, err := NewReadService(stub, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	cursor := encodeStatementCursor(statementCursor{ClientID: 2, Cutoff: now.Add(time.Minute),
		EffectiveDate: now.Add(-time.Hour), RecordedAt: now.Add(-time.Hour), Type: "payment_posted", SourceID: 1})
	if _, err := service.Statement(context.Background(), 1, StatementFilter{Cursor: cursor}); !errors.Is(err, ErrInvalidStatementCursor) {
		t.Fatalf("foreign/future cursor error = %v", err)
	}
}

func (cursor *statementCursor) EventIDForTest() string {
	if cursor == nil {
		return ""
	}
	return cursor.Type + ":" + decimal.NewFromInt(cursor.SourceID).String()
}

func int64Pointer(value int64) *int64 { return &value }

func TestStatementSQLProjectsAllEventsAndRunsBalanceBeforeCursor(t *testing.T) {
	for _, fragment := range []string{"invoice_issued", "invoice_canceled", "payment_posted", "payment_reversed",
		"created_at <= @cutoff", "reversed_at <= @cutoff", "canceled_at <= @cutoff"} {
		if !strings.Contains(statementEventsSQL, fragment) {
			t.Errorf("events SQL missing %q", fragment)
		}
	}
	if !strings.Contains(statementRunningSQL, "SUM(debit - credit) OVER") {
		t.Fatal("statement balance must be a running authoritative projection")
	}
}

package store

import (
	"context"

	storage "github.com/osr-alliance/backend-lib-storage"
)

func (s *store) SetLead(ctx context.Context, lead *Leads) error {
	return s.store.Insert(ctx, lead)
}

func (s *store) GetLeadByID(ctx context.Context, id int32) (*Leads, error) {
	l := &Leads{
		LeadID: id,
	}
	return l, s.store.Select(ctx, l, LeadsGetByID)
}

func (s *store) GetLeadsByUserID(ctx context.Context, id int32) ([]Leads, error) {
	l := &Leads{
		UserID: id,
	}

	leads := []Leads{}
	err := s.store.SelectAll(ctx, l, &leads, LeadsGetByUserID, &storage.SelectOptions{
		Limit:    -1,
		Offset:   0,
		FetchAll: true,
	})

	return leads, err
}

func (s *store) UpdateLeadsNotes(ctx context.Context, id int32, note string) (*Leads, error) {
	lead, err := s.GetLeadByID(ctx, id)
	if err != nil {
		return nil, err
	}

	lead.Notes = note

	return lead, s.store.Update(ctx, lead)
}

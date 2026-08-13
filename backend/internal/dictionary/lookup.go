package dictionary

import (
	"context"
	"strings"

	entdict "owl/backend/ent/dictionary"
	entuser "owl/backend/ent/user"
)

// ResolveAccessibleDictionaryID finds one enabled dictionary by name or title
// without loading and materializing the caller's entire accessible library.
func (s *Service) ResolveAccessibleDictionaryID(ctx context.Context, userID int, name string) (int, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}

	query := s.client.Dictionary.Query().
		Where(
			entdict.Enabled(true),
			entdict.Or(
				entdict.Public(true),
				entdict.HasOwnerWith(entuser.IDEQ(userID)),
			),
		).
		Order(entdict.ByTitle(), entdict.ByID()).
		Select(entdict.FieldID, entdict.FieldName, entdict.FieldTitle)
	items, err := query.All(ctx)
	if err != nil {
		return 0, false, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, name) || strings.EqualFold(item.Title, name) {
			return item.ID, true, nil
		}
	}
	return 0, false, nil
}

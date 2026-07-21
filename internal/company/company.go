package company

import (
	"context"
	"encoding/json"
	"fmt"

	"pelagica-studios/internal/tmdb"
)

// Details is the flattened set of fields persisted for a production company:
// the company's own attributes plus the one logo entry (if any) matching the
// company's top-level logo_path.
type Details struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Headquarters    *string  `json:"headquarters"`
	Homepage        *string  `json:"homepage"`
	Description     *string  `json:"description"`
	OriginCountry   *string  `json:"origin_country"`
	LogoFilePath    *string  `json:"logo_file_path"`
	LogoAspectRatio *float64 `json:"logo_aspect_ratio"`
	LogoHeight      *int     `json:"logo_height"`
	LogoID          *string  `json:"logo_id"`
	LogoFileType    *string  `json:"logo_file_type"`
	LogoWidth       *int     `json:"logo_width"`
	LogoVoteCount   *int     `json:"logo_vote_count"`
	LogoVoteAverage *float64 `json:"logo_vote_average"`
}

type logo struct {
	AspectRatio float64 `json:"aspect_ratio"`
	FilePath    string  `json:"file_path"`
	Height      int     `json:"height"`
	ID          string  `json:"id"`
	FileType    string  `json:"file_type"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	Width       int     `json:"width"`
}

type response struct {
	Description   string  `json:"description"`
	Headquarters  string  `json:"headquarters"`
	Homepage      string  `json:"homepage"`
	ID            int64   `json:"id"`
	LogoPath      *string `json:"logo_path"`
	Name          string  `json:"name"`
	OriginCountry string  `json:"origin_country"`
	Images        struct {
		Logos []logo `json:"logos"`
	} `json:"images"`
}

func Fetch(ctx context.Context, client *tmdb.Client, id int64) (*Details, error) {
	body, err := client.Get(ctx, fmt.Sprintf("/3/company/%d", id), map[string]string{"append_to_response": "images"})
	if err != nil {
		return nil, err
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse company %d response: %w", id, err)
	}

	details := &Details{
		ID:            parsed.ID,
		Name:          parsed.Name,
		Headquarters:  nonEmpty(parsed.Headquarters),
		Homepage:      nonEmpty(parsed.Homepage),
		Description:   nonEmpty(parsed.Description),
		OriginCountry: nonEmpty(parsed.OriginCountry),
	}

	if parsed.LogoPath != nil {
		for _, l := range parsed.Images.Logos {
			if l.FilePath != *parsed.LogoPath {
				continue
			}
			l := l
			details.LogoFilePath = &l.FilePath
			details.LogoAspectRatio = &l.AspectRatio
			details.LogoHeight = &l.Height
			details.LogoID = &l.ID
			details.LogoFileType = &l.FileType
			details.LogoWidth = &l.Width
			details.LogoVoteCount = &l.VoteCount
			details.LogoVoteAverage = &l.VoteAverage
			break
		}
	}

	return details, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

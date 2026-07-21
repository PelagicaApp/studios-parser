package db

import (
	"database/sql"

	"pelagica-studios/internal/company"
)

func ensureProductionCompaniesSchema(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS production_companies (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			headquarters TEXT,
			homepage TEXT,
			description TEXT,
			origin_country TEXT,
			logo_file_path TEXT,
			logo_aspect_ratio REAL,
			logo_height INTEGER,
			logo_id TEXT,
			logo_file_type TEXT,
			logo_width INTEGER,
			logo_vote_count INTEGER,
			logo_vote_average REAL
		)
	`)
	return err
}

func ExistingProductionCompanyIDs(database *sql.DB) (map[int64]struct{}, error) {
	rows, err := database.Query(`SELECT id FROM production_companies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

func UpsertProductionCompany(database *sql.DB, details company.Details) error {
	_, err := database.Exec(`
		INSERT OR REPLACE INTO production_companies (
			id, name, headquarters, homepage, description, origin_country,
			logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
			logo_width, logo_vote_count, logo_vote_average
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		details.ID, details.Name, details.Headquarters, details.Homepage, details.Description, details.OriginCountry,
		details.LogoFilePath, details.LogoAspectRatio, details.LogoHeight, details.LogoID, details.LogoFileType,
		details.LogoWidth, details.LogoVoteCount, details.LogoVoteAverage,
	)
	return err
}

func AllProductionCompanies(database *sql.DB) ([]company.Details, error) {
	rows, err := database.Query(`
		SELECT id, name, headquarters, homepage, description, origin_country,
		       logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
		       logo_width, logo_vote_count, logo_vote_average
		FROM production_companies
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []company.Details
	for rows.Next() {
		var d company.Details
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Headquarters, &d.Homepage, &d.Description, &d.OriginCountry,
			&d.LogoFilePath, &d.LogoAspectRatio, &d.LogoHeight, &d.LogoID, &d.LogoFileType,
			&d.LogoWidth, &d.LogoVoteCount, &d.LogoVoteAverage,
		); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

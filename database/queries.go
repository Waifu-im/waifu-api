package database

import (
	"database/sql"
	"fmt"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/lib/pq"
	"math/rand"
	"strconv"
	"time"
)

// Some methods to fetch data from the database

// FetchImages this method is charged of generating the appropriate query based on the user query params (after binding) and querying the database
func (database Database) FetchImages(
	isNsfw string,
	includedTags []string,
	excludedTags []string,
	includedFiles []string,
	excludedFiles []string,
	gif string,
	orderBy string,
	orientation string,
	many bool,
	full bool,
	userId uint,
) (ImageRows, error) {
	var parameters []any
	query := "SELECT DISTINCT Q.signature,Q.extension,Q.image_id,Q.favourites,Q.dominant_color,Q.source,Q.uploaded_at,Q.is_nsfw,Q.width,Q.height,"
	if userId != 0 {
		query += "Q.liked_at,"
	}
	query += "Tags.id as tag_id,Tags.name,Tags.description,Tags.is_nsfw as tag_is_nsfw " +
		"FROM (" +
		"SELECT Images.signature,Images.extension,Images.image_id,Images.dominant_color,Images.source," +
		"Images.uploaded_at,Images.is_nsfw,Images.width,Images.height,"
	if userId != 0 {
		query += "FavImages.liked_at,"
	}
	query += "(SELECT COUNT(image_id) from FavImages WHERE image_id=Images.image_id) as favourites " +
		"FROM Images JOIN LinkedTags ON Images.image_id=LinkedTags.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id "
	if userId != 0 {
		query += fmt.Sprintf("JOIN FavImages ON FavImages.image_id=Images.image_id AND FavImages.user_id=$%v ", len(parameters)+1)
		parameters = append(parameters, userId)
	}
	query += "WHERE not Images.under_review and not Images.hidden " + FormatNsfwTags(isNsfw, includedTags, &parameters)
	query += FormatGif(gif)
	query += FormatOrientation(orientation)
	if len(includedFiles) > 0 {
		query += fmt.Sprintf("and (Images.image_id::text ILIKE ANY($%v) or Images.signature ILIKE ANY($%v))", len(parameters)+1, len(parameters)+1)
		parameters = append(parameters, pq.Array(includedFiles))
	}
	if len(excludedFiles) > 0 {
		query += fmt.Sprintf("and not (Images.image_id::text ILIKE ANY($%v) or Images.signature ILIKE ANY($%v)) ", len(parameters)+1, len(parameters)+1)
		parameters = append(parameters, pq.Array(excludedFiles))
	}
	if len(includedTags) > 0 {
		query += fmt.Sprintf("and Tags.name ILIKE ANY($%v) ", len(parameters)+1)
		parameters = append(parameters, pq.Array(includedTags))
	}
	if len(excludedTags) > 0 {
		query += fmt.Sprintf("and NOT EXISTS (SELECT 1 FROM LinkedTags AS lk JOIN Tags T ON lk.tag_id=T.id WHERE lk.image_id = Images.image_id AND T.name ILIKE ANY($%v)) ", len(parameters)+1)
		parameters = append(parameters, pq.Array(excludedTags))
	}
	query += "GROUP BY Images.image_id"
	if userId != 0 {
		query += ",FavImages.liked_at"
	}
	query += " "
	if len(includedTags) > 0 {
		query += "HAVING COUNT(*)=" + strconv.FormatInt(int64(len(includedTags)), 10) + " "
	}
	query += FormatOrderBy(orderBy, "", true)
	if !(full || len(includedFiles) > 0) && userId == 0 && many {
		query += "LIMIT " + ManyLimit + " "
	} else if !(full || len(includedFiles) > 0) && userId == 0 {
		query += "LIMIT  1 "
	}
	query += ") AS Q JOIN LinkedTags ON LinkedTags.image_id=Q.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id "
	query += FormatOrderBy(orderBy, "Q.", false)
	imageRows := ImageRows{database.configuration, []ImageRow{}}
	rows, err := database.Db.Query(query, parameters...)
	if err != nil {
		return imageRows, err
	}
	defer rows.Close()
	for rows.Next() {
		var imageRow ImageRow
		if userId == 0 {
			err = rows.Scan(
				&imageRow.Signature,
				&imageRow.Extension,
				&imageRow.ImageId,
				&imageRow.Favourites,
				&imageRow.DominantColor,
				&imageRow.Source,
				&imageRow.UploadedAt,
				&imageRow.IsNsfw,
				&imageRow.Width,
				&imageRow.Height,
				&imageRow.TagId,
				&imageRow.Name,
				&imageRow.Description,
				&imageRow.TagIsNsfw,
			)
		} else {
			err = rows.Scan(
				&imageRow.Signature,
				&imageRow.Extension,
				&imageRow.ImageId,
				&imageRow.Favourites,
				&imageRow.DominantColor,
				&imageRow.Source,
				&imageRow.UploadedAt,
				&imageRow.IsNsfw,
				&imageRow.Width,
				&imageRow.Height,
				&imageRow.LikedAt,
				&imageRow.TagId,
				&imageRow.Name,
				&imageRow.Description,
				&imageRow.TagIsNsfw,
			)
		}
		if err != nil {
			return imageRows, err
		}
		imageRows.Rows = append(imageRows.Rows, imageRow)
	}
	err = rows.Err()
	if err != nil {
		return imageRows, err
	}
	if orderBy == Random && (full || many || len(includedFiles) > 0) {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(imageRows.Rows), func(i, j int) { imageRows.Rows[i], imageRows.Rows[j] = imageRows.Rows[j], imageRows.Rows[i] })
	}
	return imageRows, nil
}

func (database Database) ToggleImageInFav(userId uint, imageId uint) (string, error) {
	var empty uint
	err := database.Db.QueryRow("SELECT image_id FROM FavImages WHERE user_id = $1 and image_id = $2", userId, imageId).Scan(&empty)
	if err != nil {
		if err == sql.ErrNoRows {
			err = database.InsertImageToFav(userId, imageId)
			if err != nil {
				return "", err
			}
			return "INSERTED", nil
		}
		return "", err
	}
	err = database.DeleteImageFromFav(userId, imageId)
	if err != nil {
		return "", err
	}
	return "DELETED", nil

}

func (database Database) InsertImageToFav(userId uint, imageId uint) error {
	_, err := database.Db.Exec("INSERT INTO FavImages(user_id,image_id) VALUES($1,$2)", userId, imageId)
	if err != nil {
		return err
	}
	return nil
}

func (database Database) DeleteImageFromFav(userId uint, imageId uint) error {
	var empty uint
	err := database.Db.QueryRow("DELETE FROM FavImages WHERE user_id=$1 and image_id=$2 RETURNING image_id", userId, imageId).Scan(&empty)
	if err != nil {
		return err
	}
	return nil
}

func (database Database) InsertUser(user ipc.User) error {
	_, err := database.Db.Exec("INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2", user.Id, user.FullName)
	if err != nil {
		return err
	}
	return nil
}

func (database Database) Report(userId uint, imageId uint, description *string) (ReportRes, error) {
	var res ReportRes
	err := database.Db.QueryRow("INSERT INTO Reported_images (author_id,description,image_id) VALUES ($1,$2,$3) ON CONFLICT(image_id) DO UPDATE SET author_id=excluded.author_id RETURNING author_id,description,image_id, (xmax!=0) as existed", userId, description, imageId).Scan(&res.AuthorId, &res.Description, &res.ImageId, &res.Existed)
	if err != nil {
		return res, err
	}
	return res, err
}

func (database Database) GetTags() ([]Tag, error) {
	rows, err := database.Db.Query("SELECT id as tag_id, name, description, is_nsfw FROM Tags")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tagRows []Tag
	for rows.Next() {
		var tag Tag
		err := rows.Scan(&tag.TagId, &tag.Name, &tag.Description, &tag.IsNsfw)
		if err != nil {
			return tagRows, err
		}
		tagRows = append(tagRows, tag)
	}
	err = rows.Err()
	if err != nil {
		return tagRows, err
	}
	return tagRows, nil
}

func (database Database) IsValidCredentials(userId uint, secret string) (bool, error) {
	var isBlacklisted bool
	err := database.Db.QueryRow(`SELECT is_blacklisted FROM Registered_user WHERE id=$1 and secret=$2`, userId, secret).Scan(&isBlacklisted)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if isBlacklisted {
		return false, BlacklistedError
	}
	return true, nil
}

// GetMissingPermissions Provide a user id, an optional target user id, and permissions
// checks for all provided permissions that a user has the permissions globally or for the target if provided.
// Some might argue that performing a query for each permission might not be a good idea ,however fetching permissions
// position before might be a bit of a hassle and conduct to more query for the most command case : /fav , that only needs one permission.
// feel free to share your opinion if you have a better idea.
func (database Database) GetMissingPermissions(userId uint, targetUserId uint, permissions []string) ([]string, error) {
	var missing []string
	query := "SELECT user_permissions.user_id, user_permissions.target_id, permissions.position, permissions.name FROM user_permissions " +
		"JOIN permissions ON permissions.name=user_permissions.permission " +
		"JOIN registered_user on registered_user.id=user_permissions.user_id " +
		"WHERE registered_user.id=$1 " +
		"and (permissions.name=$2 or permissions.position > (SELECT position from permissions where name=$2))" +
		"and (user_permissions.target_id=$3 or user_permissions.target_id is NULL)"
	if targetUserId == userId {
		return missing, nil
	}
	userPermissions := PermissionsInformation{}
	for _, perm := range permissions {
		err := database.Db.QueryRow(query, userId, perm, targetUserId).Scan(
			&userPermissions.UserId,
			&userPermissions.TargetId,
			&userPermissions.Position,
			&userPermissions.Name,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				missing = append(missing, perm)
				continue
			}
			return missing, err
		}
	}
	return missing, nil
}

package auth

type UserResponse struct {
	ID          int64   `json:"id"`
	Email       string  `json:"email"`
	FirstName   string  `json:"firstName"`
	LastName    string  `json:"lastName"`
	DateOfBirth string  `json:"dateOfBirth"`
	AvatarPath  *string `json:"avatarPath"`
	Nickname    *string `json:"nickname"`
	AboutMe     *string `json:"aboutMe"`
	IsPublic    bool    `json:"isPublic"`
}

type UserSearchResult struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Nickname   *string `json:"nickname"`
	AvatarPath *string `json:"avatarPath"`
}

const maxUserSearchResults = 20

const maxSearchLength = 100

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
package domain

type Error string

func (e Error) Error() string {
	return string(e)
}

// Code возвращает код ответа API. Неизвестная доменная ошибка остаётся внутренней.
func (e Error) Code() string {
	switch e {
	case ErrPostNotFound, ErrCommentNotFound:
		return "NOT_FOUND"
	case ErrNotAuthor:
		return "FORBIDDEN"
	case ErrCommentsDisabled:
		return "COMMENTS_DISABLED"
	case ErrParentPostMismatch:
		return "PARENT_POST_MISMATCH"
	case ErrEmptyCommentText, ErrCommentTooLong, ErrEmptyPostTitle,
		ErrEmptyPostContent, ErrEmptyAuthor, ErrInvalidText,
		ErrInvalidID, ErrSelfParent, ErrInvalidPageSize, ErrInvalidCursor:
		return "BAD_USER_INPUT"
	default:
		return "INTERNAL"
	}
}

const (
	ErrPostNotFound       Error = "пост не найден"
	ErrCommentNotFound    Error = "комментарий не найден"
	ErrCommentsDisabled   Error = "комментарии к посту отключены"
	ErrParentPostMismatch Error = "родительский комментарий относится к другому посту"
	ErrEmptyCommentText   Error = "текст комментария пуст"
	ErrCommentTooLong     Error = "длина текста комментария превышает 2000 символов"
	ErrEmptyPostTitle     Error = "заголовок поста пуст"
	ErrEmptyPostContent   Error = "содержание поста пусто"
	ErrEmptyAuthor        Error = "идентификатор автора пуст"
	ErrInvalidText        Error = "текст должен быть корректным UTF-8 без нулевого символа"
	ErrInvalidID          Error = "некорректный или нулевой идентификатор"
	ErrInvalidTime        Error = "некорректное время создания"
	ErrSelfParent         Error = "комментарий не может быть собственным родителем"
	ErrNotAuthor          Error = "изменять настройки комментариев может только автор поста"
	ErrInvalidPageSize    Error = "размер страницы должен быть от 1 до 100"
	ErrInvalidCursor      Error = "некорректный курсор пагинации"
	ErrStorage            Error = "ошибка хранилища"
)

# avito-winter-2025

## Стандартное значение = `false`, по умолчанию не требуется больше 2х ревьюверов

```
NeedMoreReviewers bool     `json:"need_more_reviewers" db:"need_more_reviewers"`
```

### Неактивный пользователь
Неактивный пользователь не может создать PR.

### Эндпоинт `/team/add`
Добавлено security: Admin

### Эндпоинт `/users/getReview`
Не возвращает `NotFound` при передаче несуществующего пользователя, так как для этого потребовалось бы:
1. JOIN с таблицей пользователей
2. Дополнительный запрос к базе данных
Принято решение возвращать пустой список для упрощения логики.


## Расширение error code:

```
enum:
- TEAM_EXISTS
- PR_EXISTS
- PR_MERGED
- NOT_ASSIGNED
- NO_CANDIDATE
- NOT_FOUND
- INVALID_BODY
```
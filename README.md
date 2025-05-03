Проект реализует обратный прокси с балансировкой нагрузки и ограничением запросов 
по алгоритму Token Bucket. Каждому клиенту можно задать индивидуальные лимиты, 
конфигурация хранится в PostgreSQL и доступна через HTTP CRUD API.

Запуск:
git clone https://github.com/YourVeigar/cloud_task
cd cloud_task

docker compose up --build

Остановить и удалить все контейнеры:
docker compose down

Тестирование:
После запуска запустить скрипт test.sh

CRUD API для лимитов:
POST /rate-limits — создать лимит
DELETE /rate-limits — удалить лимит
GET /rate-limits — получить все лимиты
GET /rate-limits/{client_id} — получить лимит по client_id


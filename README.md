# potat-api

Public hastebin server,
public url shortening server,
private file hosting server,
and backend for chatbot [PotatBotat](https://potat.app)

### To run locally

- Clone the repository
- Ensure you're running [PostgreSQL](https://www.postgresql.org/download/), and [Redis](https://redis.io/docs/getting-started/installation/) servers
- Optionally install [ClickHouse](https://clickhouse.com/docs/en/quick-start), and [RabbitMQ](https://www.rabbitmq.com/docs/download) if enabling the potat api, and [Prometheus](https://prometheus.io/docs/prometheus/latest/installation/) if enabling metrics
- Populate `exampleconfig.json` with the database credentials, and ports for services you want to run.

### Example chatterino uploader configuration


![c03fc3](https://github.com/user-attachments/assets/b7f865ff-5432-45ab-8b9a-1d92ff99812e)

# potat-api

API for chatbot [PotatBotat](https://potat.app)

### To run locally

- Clone the repository
- Ensure you're running [PostgreSQL](https://www.postgresql.org/download/), and [Redis](https://redis.io/docs/getting-started/installation/) servers
- Optionally install [ClickHouse](https://clickhouse.com/docs/en/quick-start), and [RabbitMQ](https://www.rabbitmq.com/docs/download) if enabling the potat api, and [Prometheus](https://prometheus.io/docs/prometheus/latest/installation/) if enabling metrics
- Populate `exampleconfig.json` with the database credentials, and ports for services you want to run.

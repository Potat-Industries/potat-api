# potat-api

This repository serves multiple purposes: 
- A public hastebin server,
- A public url shortening server,
- A private file hosting server,
- The backend for chatbot [PotatBotat](https://potat.app)  

On startup if the required Postgres tables for haste, url shortener, or image hosting do not exist, they will be created. However for the PotatBotat backend, it's assumed you already have the database preconfigured.

### To run locally

- Clone the repository
- Ensure you're running [PostgreSQL](https://www.postgresql.org/download/), and [Redis](https://redis.io/docs/getting-started/installation/) servers as they are required for every service. You will need to install the [pgstd](https://github.com/grahamedgecombe/pgzstd) extension for Postgres if you intend to use the hastebin server.
- Optionally install [ClickHouse](https://clickhouse.com/docs/en/quick-start), and [RabbitMQ](https://www.rabbitmq.com/docs/download) if enabling the PotatBotat backend, and [Prometheus](https://prometheus.io/docs/prometheus/latest/installation/) if enabling metrics
- Populate `exampleconfig.json` with the database credentials, and ports for services you want to run. It will be renamed on startup.

### Example Chatterino uploader configuration


![c03fc3](https://github.com/user-attachments/assets/b7f865ff-5432-45ab-8b9a-1d92ff99812e)

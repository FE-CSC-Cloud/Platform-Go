HAIIII dit is een [GO Echo](https://echo.labstack.com/docs) project

# Development
1. Installeer [GO](https://golang.org/doc/install) volgens de gelinkte instructies.
2. Stel de env in op basis van de `.env.example` bestand.
3. Installeer [docker](https://docs.docker.com/desktop/)
4. Start de mySQL en keyDB database met `docker compose up -d`
5. Draai het project met `go run .`
    Dit download meteen alle dependencies en start het progamma.
    Dit moet je elke keer doen als je iets veranderd in de code.



   
## Waarom KeyDB?
KeyDB is een fork van Redis, het is sneller en heeft meer features dan Redis.

We gebruiken de LevelDB package om de data op te slaan in KeyDB omdat die van [keyDB deprecated](https://github.com/robaho/keydb?tab=readme-ov-file#keydb) is.
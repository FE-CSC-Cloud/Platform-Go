HAIIII dit is een [GO Echo](https://echo.labstack.com/docs) project

# Development
1. Installeer [GO](https://golang.org/doc/install) volgens de gelinkte instructies.
2. Stel de env in op basis van de `.env.example` bestand.
3. Installeer [docker](https://docs.docker.com/desktop/)
4. Start de mySQL en redis database met `docker compose up -d`
5. Draai het project met `go run .`
    Dit download meteen alle dependencies en start het progamma.
    Dit moet je elke keer doen als je iets veranderd in de code.
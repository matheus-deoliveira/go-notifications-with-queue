# Fintech Credit Notification System 🚀

Este projeto é uma prova de conceito (PoC) baseada em arquitetura orientada a eventos (Event-Driven), projetada para simular uma esteira de aprovação de crédito de ponta a ponta, comumente encontrada em bancos e grandes fintechs.

O foco principal deste repositório é demonstrar o manuseio crítico de transações assíncronas através de **resiliência**, **idempotência** e proteção contra perda de dados utilizando **Dead Letter Queues (DLQ)**.

## 🏗 Arquitetura e Casos de Uso

A aplicação é dividida em dois microserviços/comandos principais:

1. **Producer (`producer/main.go`)**: Simula o "Motor de Crédito". Quando uma proposta de financiamento é aprovada, ele emite um evento (JSON) confirmando a aprovação.
2. **Worker (`worker/main.go`)**: O consumidor resiliente. Ele capta o evento assíncrono e tem o dever de enviar o contrato (simulado) para o cliente. 

### 🛡 Recursos "Enterprise-Grade" Implementados

* **Dead Letter Queue (DLQ) Automática**: Se o provedor de e-mail cair e a mensagem falhar após 3 tentativas de re-processamento, a mensagem não é perdida. Ela sofre `Nack` (rejeição) e o RabbitMQ a roteia automaticamente para a fila `credit_approved_dlq` via `x-dead-letter-exchange`. Isso permite intervenção manual do time de suporte sem perda financeira.
* **Idempotência na Prática**: Em sistemas distribuídos, uma mensagem pode ser entregue mais de uma vez (At-Least-Once delivery). O Worker verifica o `EventId` na memória antes de processar. Se repetido, o sistema apenas reconhece a mensagem (Ack) silenciosamente evitando enviar dois contratos ao mesmo cliente.
* **Retries com Backoff Controlado**: Trabalhadores não travam a fila principal caso um erro temporário ocorra. Eles anotam a falha nos `Headers` da mensagem (ex: `x-retry-count`) e enfileiram novamente.
* **Topologia RabbitMQ via Go**: Declaração programática de filas secundárias, *Bindings* e *Exchanges* diretamente no código de consumo, garantindo que a infraestrutura suba corretamente de forma autônoma.

## 🛠 Tecnologias
* **Go (Golang)**: Alta concorrência e simplicidade.
* **RabbitMQ**: Message Broker puro utilizando o protocolo padrão AMQP 0-9-1.
* **Docker Compose**: Para portabilidade da infraestrutura de filas localmente.

---

## 🚀 Como Executar o Projeto

### 1. Subir a Infraestrutura (RabbitMQ)
Certifique-se de ter o Docker instalado e inicie o container:
```bash
docker-compose up -d
```
> Painel de Controle Web: http://localhost:15672 (Usuário: `guest`, Senha: `guest`)

### 2. Iniciar o Microserviço Consumidor (Worker)
Abra um terminal e mantenha o Worker rodando, escutando a fila:
```bash
go run worker/main.go
```

### 3. Disparar o Evento (Producer)
Em uma nova aba do terminal, simule o motor de crédito gerando um evento aprovado:
```bash
go run producer/main.go
```

### 4. Observe a Mágica Acontecer
No terminal do `worker`, você verá o sistema tentar disparar o e-mail 3 vezes. Ao falhar sequencialmente (simulado intencionalmente no código para testes), a mensagem será blindada e enviada para a DLQ. Você pode verificar a mensagem morta lá através da UI web do RabbitMQ.

---

## 🔮 Próximos Passos (Roadmap de Evolução)

Para deixar este repositório ainda melhor e próximo do limite de estresse de transações de um banco, os seguintes passos farão parte da evolução da arquitetura:

### 1. Simulação de Carga (Bulk Processing)
* **O que é:** Modificar o `producer` para disparar rajadas de centenas ou milhares de contratos aprovados simultaneamente.
* **Objetivo:** Simular o fechamento de lote ao final do dia comercial de um banco, enchendo a fila com alto fluxo de eventos.

### 2. Processamento Concorrente de Alta Performance (Worker Pools)
* **O que é:** Em Go, não usamos "Multi-thread" tradicionais pesadas (como em Java ou C#), mas sim **Goroutines** (threads super leves gerenciadas pela própria linguagem).
* **Objetivo:** Atualmente, o worker processa uma mensagem inteira antes de pegar a próxima (bloqueio síncrono). A ideia é implementar o design pattern **Worker Pool**. Vamos ler mensagens da fila em uma rotina principal (Router) e distribuir para um pool de N goroutines (ex: 50 workers paralelos) que enviarão os "e-mails" independentemente no background.

### 3. Rate Limiting e Tuning de QoS (Prefetch Count)
* **O que é:** Ao ligar milhares de workers e injetar volume, se não dissermos ao RabbitMQ para segurar a velocidade, ele tentará dar "push" em todas as mensagens para o consumidor Go injetando tudo na RAM de uma vez, o que geraria um pico de memória (Out of Memory).
* **Objetivo:** Configurar o limite `QoS (Quality of Service) / Prefetch Count` para, por exemplo, 100. Assim, o RabbitMQ só enviará no máximo 100 mensagens não reconhecidas por vez. Assim que o Go finalizar uma entrega de e-mail e der o `Ack`, o Rabbit envia mais uma, criando uma "torneira controlada" imune a quedas por falta de memória.

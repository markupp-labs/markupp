# Pesquisa de Cobrança

Status: proposta para apresentação. A cobrança ainda não está implementada.

Este documento reúne a pesquisa de mercado, as medições de custo e a recomendação de planos
do markupp. Cada seção abre com a conclusão principal, para facilitar a montagem dos slides.

## Resumo

- O markupp cobra por organização (tenant), não por pessoa. Agentes de IA não pagam.
- Quatro planos: Free, Pro ($8), Max ($39 por time) e Enterprise ($15 por pessoa).
- O custo de operar um cliente é baixo. No pior caso, os planos pagos ficam com cerca de
  65% de margem.
- Nenhum concorrente junta organização automática, markdown aberto e acesso para agentes de
  IA em todos os planos. Esse é o diferencial.
- A venda acontece em dólar, pela Paddle, que cuida dos impostos de todos os países.

## 1. O mercado

Conclusão: uma pessoa paga entre $8 e $20 por mês por um app de notas com IA. Um time de 10
pessoas paga entre $100 e $200 por mês nas ferramentas mais conhecidas.

### Notas pessoais com IA

| Produto | Preço mensal | Como cobra | Observação |
|---|---|---|---|
| Obsidian | Sync de $4 a $10, Publish $8 por site | por pessoa | sem IA própria |
| Capacities | $9,99 a $12,49 | por pessoa | IA com cota não informada |
| Heptabase | $8,99 a $53,99 | por pessoa | IA por créditos, sem plano grátis |
| Saner.ai | $8 a $16 | por pessoa | limite de notas no plano de entrada |
| Mem | $9 a $199 | por cota de uso | limita até as notas novas por mês |
| Tana | $20 a $30 | por pessoa | acesso para agentes só no plano pago |

### Base de conhecimento para times

| Produto | Preço por pessoa | Time de 10 pessoas |
|---|---|---|
| Notion Business | $20 | $200 |
| Slite | $10 a $20 | $100 a $200 |
| Basic Memory (Team) | $15 | $150 |
| Confluence | cerca de $5 a $10 | $50 a $100 |
| Outline | preço fixo por faixa de time | $10 (até 10 pessoas) |

### Memória para agentes de IA

| Produto | Preço mensal | Como cobra |
|---|---|---|
| Supermemory | $0, $19, $100, $399 | créditos em dólar |
| Mem0 | $0, $19, $249 | por requisição |
| Zep | $0, $125, $375 | por créditos |

### Tendências que apoiam o modelo escolhido

- O modelo misto, com preço fixo e limite de uso, é o mais adotado em 2026: 37% das 230
  empresas ouvidas no levantamento de Kyle Poyar.
- 42% dos produtos já oferecem alguma cobrança por uso, contra 27% em 2023 (Stripo, 2026).
- Produtos de código aberto com versão gerenciada cobram por organização, e não por pessoa.
  Langfuse ($0, $29, $199) e Supabase ($0, $25, $599) seguem esse modelo, que é o mesmo do
  markupp.

## 2. O espaço livre no mercado

Conclusão: quem organiza as notas com IA prende os dados do cliente, e quem usa formato
aberto não organiza nada sozinho. O markupp ocupa o meio.

O que a pesquisa encontrou:

1. **Organização automática com formato aberto.** Mem, Heptabase e Notion organizam, mas
   guardam as notas num formato próprio. Obsidian, Logseq e Basic Memory usam markdown, mas
   deixam a organização com o usuário.
2. **Acesso para agentes de IA costuma ser caro.** O Supermemory oferece esse acesso só no
   plano Enterprise e o Tana só no plano pago. Notion e Outline não citam o recurso.
3. **Créditos de IA são difíceis de entender.** Notion, Confluence, Heptabase e Capacities
   vendem IA por créditos. O markupp não precisa disso, porque não roda um modelo de
   linguagem: o cliente usa o assistente que já tem (Claude, ChatGPT e outros).
4. **Poucos cobram por time.** Só Outline e Craft cobram um valor fixo pelo grupo, e nenhum
   dos dois tem acesso para agentes de IA.
5. **Instalação própria gratuita é rara.** O markupp pode ser instalado de graça (licença
   AGPL), o que dá segurança a quem tem receio de ficar preso a um fornecedor.

Proposta de frase para o produto: *"A knowledge base that organizes itself, for you and your
agents. Your markdown, any AI, no credits."*

## 3. Medições de custo

Conclusão: o custo por nota é pequeno e o que pesa é a memória do banco de dados, não a IA.

### Como medimos

Todas as medições rodaram em um notebook com WSL2, 12 núcleos e 5 GB de memória, usando
Docker. Os scripts, os resultados e os passos para reproduzir estão em
[pesquisa/](pesquisa/README.md).

- **Notas reais:** 6.498 arquivos markdown da documentação pública do Obsidian, em vários
  idiomas, mais as 59 páginas da documentação do markupp.
- **Sistema real:** o markupp foi iniciado pelo `docker compose` do projeto. As notas entraram
  pela API e o indexador processou todas elas.
- **Busca por sentido:** o markupp ainda não gera vetores. Usamos PostgreSQL 17 com pgvector
  0.8.1, a mesma base do projeto, com 10 mil e 100 mil vetores de 1024 posições, que é o
  formato do modelo escolhido (Amazon Titan v2).
- **Contagem de texto:** as notas foram divididas em tokens (pedaços de palavra que os
  provedores de IA usam para cobrar).

As notas da documentação do Obsidian são maiores que as de uso pessoal: cerca de 620 palavras
contra 175 a 400 em cofres reais. Por isso, os custos abaixo estão acima do real.

### Resultados

| O que foi medido | Resultado |
|---|---|
| Tamanho de uma nota em tokens | de 715 a 1.443 tokens |
| Espaço no banco por nota, hoje | 35 KB, 6,7 vezes o tamanho do texto original |
| Espaço extra para a busca por sentido | 8,3 KB por trecho, com 1,8 a 3,2 trechos por nota |
| Velocidade de gravação | 139 notas por segundo |
| Tempo para organizar 6.498 notas | 4 minutos e 30 segundos |
| Busca por palavra na API | 139 ms (mediana) e 174 ms (95% das buscas) |
| Busca por sentido em 100 mil trechos | 1,6 ms (mediana) e 2,6 ms (95% das buscas) |
| Memória sem uso | API 51 MB, indexador 238 MB, banco 443 MB |

### Uma escolha técnica que reduz o custo

Guardar os vetores com metade da precisão (formato halfvec do pgvector) reduziu o índice de
busca a um terço do tamanho:

| Formato | Tamanho do índice com 100 mil trechos | Acerto | Tempo de busca |
|---|---|---|---|
| Precisão completa | 781 MB | 98% | 1,9 ms |
| Metade da precisão | 260 MB | 100% | 1,6 ms |

Não houve perda de qualidade nem de velocidade, então a recomendação é usar metade da
precisão.

### Custo mensal no pior caso

O cálculo considera o plano com o limite de notas cheio, os índices sempre em memória e 10%
das notas editadas cinco vezes por mês. Preços usados:

- IA para busca por sentido: $0,02 por milhão de tokens (Amazon Titan v2);
- memória: $6 por GB por mês (servidor de 16 GB da DigitalOcean a $96);
- disco: $0,08 por GB por mês (Amazon EBS).

| Plano | Limite de notas | Custo no pior caso | Receita líquida (anual) | Margem |
|---|---|---|---|---|
| Free | 1.000 | $0,13 | $0 | não se aplica |
| Pro | 20.000 | $2,57 | $7,56 | 66% |
| Max | 100.000 | $12,87 | $37,01 | 65% |

A receita líquida já desconta a taxa da Paddle (5% mais $0,50 por cobrança). Na assinatura
mensal, a receita líquida é de $9,00 no Pro e de $46,05 no Max.

Importar 20 mil notas de uma vez custa $0,58 em IA, e 100 mil notas custam $2,89.

## 4. Planos recomendados

Conclusão: preço de entrada abaixo da média do mercado, com preço fixo por time e sem
créditos de IA.

| | Free | Pro | Max | Enterprise |
|---|---|---|---|---|
| Para quem | conhecer o produto | uma pessoa | um time | empresas e parceiros |
| Preço anual (por mês) | $0 | $8 | $39 por time | $15 por pessoa |
| Preço mensal | $0 | $10 | $49 por time | contrato anual |
| Pessoas incluídas | 1 | 1 | 10, mais $5 por pessoa até 50 | mínimo de 25 |
| Limite de notas | 1.000 | 20.000 | 100.000 | por contrato |
| Busca por sentido e organização automática | sim | sim | sim | sim |
| Acesso para agentes de IA e API | sim | sim | sim | sim |
| Cofres públicos | sim | sim | sim | sim |
| Leitores e agentes | sem custo | sem custo | sem custo | sem custo |
| Recursos extras | | mais cofres | papéis e cofres compartilhados | login corporativo (SSO), registro de auditoria, acordo de nível de serviço, suporte à instalação própria |

### Por que esses valores

- **Pro a $8:** fica no início da faixa de notas com IA ($8 a $20) e já inclui acesso para
  agentes de IA, que outros cobram à parte.
- **Max a $39 por time:** 10 pessoas pagam $39, contra $100 a $200 nos concorrentes.
- **Enterprise por pessoa:** é o formato que as áreas de compras das empresas conhecem. O
  valor de $15 fica abaixo do Basic Memory Business ($30), e o mínimo de 25 pessoas ($375 por
  mês) cobre o custo do atendimento comercial.
- **Limites por notas:** o número de notas é o que mais pesa no custo, e é fácil de entender.
  Para impedir abuso por agentes, há um limite de requisições por minuto, sem cobrança extra.

## 5. Cofres públicos

Conclusão: qualquer plano pode publicar um cofre para leitura aberta, e a documentação do
markupp será o primeiro.

- Qualquer pessoa lê, busca e conecta o próprio agente de IA a um cofre público, sem criar
  conta.
- As notas do cofre público contam no limite do plano de quem publicou.
- O acesso aberto tem limite de requisições por minuto, para controlar o custo.
- Na apresentação, o cofre público com a documentação do markupp serve como demonstração ao
  vivo.

O Obsidian Publish e as páginas públicas do Notion mostram que conteúdo aberto atrai novos
usuários sem custo de marketing. No markupp, o cofre já é a unidade de acesso (ADR-0024),
então a publicação acontece por cofre. A leitura sem conta ainda precisa de um ADR.

## 6. Como receber os pagamentos

Conclusão: vender em dólar pela Paddle, que aceita empresas brasileiras e cuida dos impostos
de todos os países.

A Paddle atua como revendedora oficial (Merchant of Record): ela vende ao cliente final,
recolhe os impostos de cada país e repassa o valor à empresa.

| Opção | Aceita empresa brasileira | Cuida dos impostos de outros países | Real e Pix para brasileiros | Taxa |
|---|---|---|---|---|
| Paddle | sim | sim | sim, com Pix Automático | 5% + $0,50 |
| Dodo Payments | sim | sim | sim | 4% + $0,40, mais adicionais |
| Stripe (conta brasileira) | sim | não | Pix só avulso | cerca de 6,7% + câmbio |
| Lemon Squeezy, Polar e Stripe Managed Payments | não | sim | não se aplica | não se aplica |

Por que a Paddle:

- **Impostos de outros países:** a Stripe com conta brasileira não calcula impostos do
  exterior, e a empresa teria que fazer isso sozinha.
- **Clientes brasileiros:** a Paddle aceita real e Pix Automático, o que evita os 3,5% de IOF
  do cartão internacional.
- **Plano anual com desconto:** a taxa fixa de $0,50 pesa menos numa cobrança anual.
- **Plano B:** a Dodo Payments, que é mais barata, mas é uma empresa mais nova.

No Brasil, a receita de clientes do exterior costuma ter isenção de ISS, PIS e COFINS e IOF
zero no câmbio, por ser exportação de serviço. A venda a clientes brasileiros por meio da
Paddle precisa da análise de um contador antes de ser colocada em prática.

## 7. Limites desta pesquisa

- Os vetores foram gerados artificialmente, porque o markupp ainda não gera vetores. O custo
  de IA saiu do preço publicado por token, não de uma fatura real.
- O custo de memória usa como referência o preço de um servidor completo, que inclui
  processador e disco. Isso deixa o número acima do real.
- As medições mostram que o preço cobre o custo, mas não mostram quanto o cliente aceita
  pagar. Para isso, o próximo passo é entrevistar de 5 a 8 pessoas de cada público (uma
  pessoa sozinha e times), com as quatro perguntas de preço de Van Westendorp.
- Hoje a produção roda no cluster do IFSC, sem custo. Os valores em dólar simulam a operação
  comercial.

## Fontes

### Concorrentes

- Obsidian: <https://obsidian.md/pricing>
- Capacities: <https://capacities.io/pricing>
- Heptabase: <https://heptabase.com/pricing>
- Saner.ai: <https://saner.ai/pricing>
- Mem: <https://get.mem.ai/pricing>
- Tana: <https://tana.inc/pricing>
- Notion: <https://www.notion.com/pricing>
- Slite: <https://slite.com/pricing>
- Outline: <https://www.getoutline.com/pricing>
- Basic Memory: <https://basicmemory.com/pricing>
- Confluence, Guru e Slite (comparativo): <https://www.eesel.ai/blog/confluence-vs-guru-vs-slite>
- Supermemory: <https://supermemory.ai/pricing>
- Mem0: <https://mem0.ai/pricing>
- Zep: <https://www.getzep.com/pricing>
- Langfuse: <https://langfuse.com/pricing>
- Supabase: <https://supabase.com/pricing>

### Tendências de preço

- Modelos de preço em SaaS (Userpilot, com o levantamento de Kyle Poyar):
  <https://userpilot.com/blog/saas-pricing-models/>
- Tendências de preço em SaaS 2026 (Stripo): <https://research.stripo.email/saas-pricing-trends-2026>
- Modelos de preço para IA em 2026 (Monetizely):
  <https://www.getmonetizely.com/articles/the-beginners-cheat-sheet-to-ai-pricing-models-in-2026-e4939>

### Custos

- Amazon Titan Text Embeddings v2:
  <https://aws.amazon.com/blogs/machine-learning/get-started-with-amazon-titan-text-embeddings-v2-a-new-state-of-the-art-embeddings-model-on-amazon-bedrock/>
- Amazon Bedrock: <https://aws.amazon.com/bedrock/pricing/>
- Amazon EBS: <https://aws.amazon.com/ebs/pricing/>
- DigitalOcean: <https://www.digitalocean.com/pricing/droplets>
- pgvector e o formato halfvec: <https://github.com/pgvector/pgvector>
- Desempenho do índice HNSW (Supabase): <https://supabase.com/blog/increase-performance-pgvector-hnsw>
- Tamanho de cofres no Obsidian (fórum): <https://forum.obsidian.md/t/how-many-notes-do-you-have/36987>
- Notas usadas na medição: <https://github.com/obsidianmd/obsidian-help>

### Pagamentos e impostos

- Paddle, preços: <https://www.paddle.com/pricing>
- Paddle, países atendidos:
  <https://www.paddle.com/help/start/intro-to-paddle/which-countries-are-supported-by-paddle>
- Paddle e Pix: <https://developer.paddle.com/concepts/payment-methods/pix>
- Paddle no Brasil: <https://www.paddle.com/blog/payments-in-brazil>
- Dodo Payments: <https://dodopayments.com/pricing>
- Stripe Brasil: <https://stripe.com/br/pricing>
- Stripe e Pix: <https://docs.stripe.com/payments/pix>
- Países atendidos pelo Stripe Tax: <https://docs.stripe.com/tax/supported-countries>
- Stripe Managed Payments: <https://docs.stripe.com/payments/managed-payments/eligibility>
- Lemon Squeezy, atualização de 2026: <https://www.lemonsqueezy.com/blog/2026-update>
- Polar: <https://polar.sh/resources/pricing>
- Isenção de PIS, COFINS e IOF na exportação de serviços:
  <https://www.reformatributaria.com/economia-reforma-tributaria-impactos/receita-federal-detalha-isencao-de-pis-pasep-cofins-e-iof-em-exportacao-de-servicos/>
- IOF zero na exportação: <https://www.felsberg.com.br/iof-zero-na-exportacao/>
- IOF de 3,5% no cartão internacional: <https://www.infomoney.com.br/?p=2879011>

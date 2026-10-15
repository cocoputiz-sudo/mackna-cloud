# MackNa Cloud ☁️

**Plataforma independente de hospedagem, inspirada no fluxo de trabalho do Railway.** Construída para operar inicialmente numa TV Box BTV11 com Debian 13 ARM64 e crescer para múltiplos servidores Linux.

> **Estado: MVP de observabilidade e PostgreSQL Lab homologado na BTV11. Continua sendo uma plataforma de desenvolvimento, não um serviço de hospedagem pública ou sistema de produção.** O Railway original da IMORTAIS continua operando independentemente.

## Situação verificada em 14/10/2026

A infraestrutura abaixo **já foi homologada no equipamento**, mas o painel e o controlador MackNa são outro software e estão sendo desenvolvidos neste repositório:

| Componente | Estado |
| --- | --- |
| BTV11 / Amlogic S905X3 / ARM64, ~1,8 GiB RAM | Debian 13 na eMMC Biwin (~14,7 GiB) |
| Docker 29.9 / Compose / OverlayFS | Funcionando |
| PostgreSQL 16 (homologação IMORTAIS) | Healthy |
| Backend Node.js (homologação IMORTAIS) | Healthy |
| API de saúde da IMORTAIS | HTTP 200 em modo preview isolado |
| Controlador MackNa | Em execução na BTV11; autenticação por token e túnel SSH |
| Painel de projetos e serviços | Implementado, somente leitura |\n| PostgreSQL Manager Lab (Fase A) | Implementado e interface testada com dados fictícios, somente leitura |\n| Atualizador GitHub Releases | Implementado no código, **não habilitado na BTV11** |\n| Deploy GitHub, banco multi-instância, backups/restore, domínios/HTTPS, múltiplos nós | Planejados e **não disponíveis** |

A prova de infraestrutura está em [imortais-cta-bot, PR #84](https://github.com/cocoputiz-sudo/imortais-cta-bot/pull/84). **Não copiar tokens, bancos ou variáveis do Railway de produção.**

## MVP proposto

- **Controlador leve em Go**, compilado para Linux ARM64 fora da BTV11;
- API administrativa autenticada, inicialmente apenas para **consulta** de saúde, memória, armazenamento, uptime e contêineres;
- Interface web própria e responsiva, acessível inicialmente **somente por túnel SSH**;
- Operações administrativas mutáveis (deploy, restart, update, rollback) apenas em fases posteriores, com auditoria, validação de alvos e permissões;
- Evolução para projetos, pipelines GitHub Actions/GHCR, PostgreSQL gerenciado, backups, domínios HTTPS, vários nós e agentes de IA com escopo mínimo.

## Princípios de segurança

1. Não expor a API administrativa à internet ou à rede local sem TLS e autenticação adequados. Primeira versão escuta em `127.0.0.1`.
2. Nunca versionar credenciais, chaves privadas, tokens, `.env` ou cópias de bancos reais.
3. O acesso ao Docker Socket, mesmo montado como `read-only`, normalmente permite operações privilegiadas. O plano de controle **não deve oferecer shell/exec arbitrário** nem acesso direto ao socket ao navegador.
4. Não executar builds pesados na BTV11; preferir GitHub Actions para binários e imagens ARM64.
5. Persistência importante exige backup restaurável, teste de recuperação e monitoramento de desgaste do armazenamento. A eMMC da TV Box não é considerada storage de produção validado.
6. A MackNa **não migra nem altera automaticamente** os serviços hospedados atualmente no Railway.

## Roadmap

| Fase | Entrega | Situação |
| --- | --- | --- |
| 0 | Homologar Debian e Docker no Node 01 | Concluída |
| 1 | Agente Go de observabilidade + testes + build ARM64 | Homologado no Node 01 |
| 2 | Dashboard web autenticado | Homologado via túnel SSH |
| 3 | Inventário de projetos | Implementado (somente leitura) |\n| 3b | Deploy GitHub Actions/GHCR com rollback isolado | Não implementado |
| 4 | PostgreSQL Lab (métricas e tabelas) | Homologado, somente leitura |\n| 4b | Provisionamento, backups e restauração | Não implementado |
| 5 | HTTPS, domínios e publicação de aplicações | Planejado |
| 6 | Multi-node + integração administrativa com IA | Planejado |

## Repositórios

- **Este repositório**: produto MackNa Cloud, separado das aplicações hospedadas.
- **IMORTAIS**: [imortais-cta-bot](https://github.com/cocoputiz-sudo/imortais-cta-bot), usada apenas como aplicação real para homologação do nó.

## Colaboração e desenvolvimento

Trabalhar em branches, abrir PRs com testes e documentar as alterações. Não usar o estado de uma imagem Docker como prova de health sem validar os endpoints. Não implantar atualizações de controle na BTV11 sem confirmação operacional explícita.

O primeiro marco de aceitação foi atingido: o Node 01 e os contêineres existentes são visualizados sem interferir na execução da IMORTAIS. Para novas versões, o pacote precisa ter build SHA e checksum conferidos, e o instalador deve garantir rollback apenas do Agent. A plataforma não tem SLA, proteção multi-tenant ou suporte a terceiros nesta fase.

**Projeto pessoal de engenharia em evolução, não afiliado à Railway.**

## Instalação e atualização

A instalação consolidada está documentada em [Runbook Node 01](docs/node01-tomorrow-install.md). O artefato deve vir de um workflow concluído e incluir `dist/SHA256SUMS`, `dist/BUILD_INFO.txt` e `scripts/install-verified-bundle.sh`. **Nunca executar o binário apenas pelo nome ou pela versão 0.3.0.**

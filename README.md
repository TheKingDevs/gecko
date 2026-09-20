# Gecko

![Gecko](doc/assets/logo.png)

**Gecko** é uma linguagem de programação moderna baseada no ecossistema e na infraestrutura do Go, projetada para oferecer uma experiência de desenvolvimento mais simples, expressiva e produtiva, mantendo o desempenho e a confiabilidade de sua base.

O Gecko adiciona uma sintaxe mais amigável e recursos de alto nível à base do Go, permitindo desenvolver desde pequenos scripts e ferramentas de linha de comando até aplicações e sistemas de maior escala.

✨ Recursos

O Gecko inclui recursos próprios construídos sobre a base do Go, incluindo:

- Sintaxe ".gk" para arquivos Gecko.
- Execução direta com "gecko".
- "print" e "println" nativos.
- "input()" nativo para entrada de dados.
- Classes e herança.
- Sintaxe moderna para estruturas e valores.
- Modo dinâmico para uma experiência de programação mais simples.
- Modo tipado para maior controle sobre tipos e recursos avançados.
- Compatibilidade com o ecossistema de módulos do Go.
- Gerenciador de pacotes próprio através do "gpm".
- Compilação para executáveis nativos.
- Ferramentas próprias de formatação, execução e gerenciamento de projetos.

🚀 Primeiros passos

Um programa Gecko pode ser criado em um arquivo ".gk":

println("Hello, Gecko!")

Execute diretamente com:

gecko hello.gk

Ou execute o projeto atual:

gecko run .

O Gecko também possui seu próprio gerenciador de pacotes:

gpm init
gpm install <package>
gpm run

📦 Gerenciador de pacotes

O GPM (Gecko Package Manager) é o gerenciador de pacotes oficial do Gecko.

Ele fornece recursos para:

- Criar projetos Gecko.
- Instalar e remover dependências.
- Utilizar pacotes hospedados no GitHub.
- Utilizar pacotes compatíveis com o ecossistema Go.
- Gerenciar versões e dependências.
- Executar comandos definidos pelo projeto.
- Manter um arquivo de lock para builds reproduzíveis.

Exemplo:

gpm init
gpm install github.com/example/package
gpm run

🦎 Gecko e Go

O Gecko é construído a partir da base do Go e mantém compatibilidade com partes importantes do seu ecossistema, incluindo ferramentas, runtime e módulos.

Entretanto, o Gecko possui sua própria linguagem, sintaxe, ferramentas e experiência de desenvolvimento.

O objetivo não é substituir o funcionamento interno eficiente do Go, mas construir uma camada de linguagem mais produtiva sobre essa fundação.

🛠️ Compilando o Gecko

Para compilar o Gecko a partir do código-fonte, consulte a documentação de desenvolvimento do projeto.

O processo de build gera as principais ferramentas do Gecko, incluindo:

bin/
├── gecko
├── gkfmt
└── gpm

O executável principal é:

gecko

Por exemplo:

gecko run main.gk
gecko build
gecko version

📁 Estrutura do projeto

O código-fonte principal do Gecko está organizado em torno da base do compilador e runtime derivados do Go, juntamente com as extensões específicas da linguagem Gecko.

As extensões da linguagem são implementadas diretamente no código-fonte do compilador e runtime quando necessário.

Exemplos de áreas importantes:

src/
├── cmd/
│   ├── compile/
│   └── gecko/
├── runtime/
└── ...

🧪 Exemplos

Exemplos da linguagem podem ser encontrados em:

examples/
├── 00_hello.gk
└── ...

Um exemplo simples:

var name = input("Qual é o seu nome? ")
println("Olá, " + name + "!")

🤝 Contribuindo

Contribuições para o Gecko são bem-vindas.

Antes de contribuir, consulte a documentação do projeto e as diretrizes de desenvolvimento.

Ao contribuir:

- Mantenha o código consistente com a arquitetura existente.
- Escreva código e comentários em inglês.
- Adicione testes para novos recursos.
- Atualize os exemplos quando uma nova funcionalidade da linguagem for introduzida.
- Verifique se o projeto continua compilando corretamente após alterações no compilador ou runtime.

## 📄 Licença

O Gecko é distribuído sob os termos definidos em [LICENSE](LICENSE.md).

---

<div align="center">
	Gecko — uma linguagem moderna construída sobre uma fundação rápida, confiável e eficiente.
</div>

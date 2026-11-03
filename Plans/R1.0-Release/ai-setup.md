# AI-assisted installation and agent setup

📌 **TL;DR:** Give an AI one Markdown workflow: it does most installation work, then asks the questions needed to connect useful agents. The Markdown workflow is R1 work; Web Face chat is separate R1.1 work.

## Release scope

Owner request, 2026-11-03: an AI Markdown install/config/update script and AI chat in the Web Face, backed by an `ab-*` session like the owner's mdhouse integration.

The goal is a working bus and configured agents without making users translate documentation into shell commands. T-33 is in progress; this note defines journeys, not a new protocol or message format.

## Markdown workflow

- One Markdown entry point covers installing, configuring and updating AgentBus. The AI checks what exists, performs the applicable documented steps and checks the resulting node.
- Do most work automatically. Ask when a user choice or missing account detail is needed, instead of asking the user to run commands the AI can execute.
- After installation, ask which agents the user wants. Help configure email accounts (IMAP/SMTP), Telegram and other selected bundled agents using their existing help and configuration declarations.
- Help place settings and credentials in the existing Configuration/Secret homes, load the agent and confirm it answers. Finish with what works and any remaining user action.
- Apply the same workflow to existing installations and updates. An incomplete step stays incomplete in the summary.

Link the canonical [installation and update rules](../../docs/07-setup.md#release-installation-details), [runner operations](../../docs/runner/operations.md#running-and-checking-the-runner), [Configuration/Secret split](../../docs/03-records-agent.md#configuring-a-template) and [agent guides](../../docs/agents/README.md#choose-an-agent). Do not maintain a second installation contract.

## Web Face chat

T-34 moved to [R1.1](../R1.1/ai-setup.md#web-face-chat) by the owner, 2026-11-03. It reuses T-33 and does not block R1.

## Work and acceptance

[T-33](TODO.md#ai-assisted-installation-and-agent-setup) tracks the Markdown deliverable and end-user acceptance. This plan does not authorize publication, provider messages or changes to a live installation; use an isolated test system for acceptance.

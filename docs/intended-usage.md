# Intended Usage

<- [Back to README](../README.md)

---

This page explains how gentle-ai is meant to be used. Not the flags, not the architecture -- just the mental model. If you read one page besides the README, make it this one.

---

## After Installing -- You're Done

Once you run `gentle-ai` and select your agent(s), components, and preset, everything is configured. There is nothing else to do. No commands to memorize, no workflows to learn, no config files to edit.

Open your AI agent and start working. That's it.

---

## Engram (Memory) -- Don't Touch It

Engram is persistent memory for your AI agent. It saves decisions, discoveries, bug fixes, and context across sessions -- automatically. The agent manages all of it.

You never need to configure it, inspect it, or interact with it directly. If engram is working correctly, you won't even notice it's there. That's the point.

---

## SDD (Spec-Driven Development) -- It Happens Organically

SDD is a structured planning workflow for substantial features. It has phases (explore, propose, spec, design, implement, verify), but you do NOT need to learn any of them.

Here's how it actually works:

- **Small request?** The agent just does it. No ceremony.
- **Substantial feature?** The agent will suggest using SDD to plan it properly -- exploring the codebase, proposing an approach, designing the architecture, then implementing step by step.
- **Want SDD explicitly?** Just say "use sdd" or "hazlo con sdd" and the agent starts the workflow.

The agent handles all the phases internally. You just review and approve at key decision points.

---

## Multi-mode SDD -- OpenCode Only

Multi-mode lets you assign different AI models to different SDD phases -- for example, a powerful model for design and a faster one for implementation. This is an OpenCode-exclusive feature.

For **all other agents** (Claude Code, Cursor, Gemini CLI, VS Code Copilot), SDD runs in single-mode automatically. One model handles everything, and that works perfectly fine.

If you want multi-mode in OpenCode:

1. Connect your AI providers in OpenCode first
2. Run the gentle-ai installer and select "multi" when prompted

If no providers are connected, you will only see single-mode as an option.

---

## Skills -- They Load Automatically

The curated skill library (React 19, Angular, TypeScript, testing patterns, etc.) is installed to your agent's skills directory during setup. The agent detects what you're working on and loads the relevant skills automatically.

You don't need to activate, invoke, or even think about them.

**One exception: the skill registry.** After installing gentle-ai (or after adding/removing skills), it's a good idea to run `/skill-registry` in your agent. This builds a catalog of all available skills so the orchestrator knows exactly what's available and where. You only need to do it once per setup change -- after that, the agent uses it automatically.

---

## The Golden Rule

Gentle AI is an ecosystem **configurator**. It sets up your AI agent with memory, skills, workflows, and a persona -- then gets out of the way.

The less you think about gentle-ai after installing, the better it's working.

---

## Quick Reference

| Do | Don't |
|----|-------|
| Run the installer, pick your agents and preset | Manually edit the generated config files |
| Just start coding with your AI agent | Memorize SDD phases or commands |
| Let the agent suggest SDD when a task is big enough | Force SDD on every small task |
| Trust that engram is saving context for you | Dig into engram's storage or try to manage it |
| Run `/skill-registry` after installing or changing skills | Forget to update the registry after adding new skills |
| Say "use sdd" if you know you want structured planning | Worry about which SDD phase comes next |
| Re-run the installer to update or change your setup | Manually patch skill files or persona instructions |

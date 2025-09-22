# Custom Prompt Configuration

This directory contains prompt configuration files for the Blackbox summarisation system. You can create custom prompts by adding JSON files to this directory.

## Default Prompts

- **meeting.json** - Comprehensive meeting summarisation with executive summary, themes, decisions, and action items
- **dictation.json** - Focused summarisation for single-speaker dictation and personal notes
- **technical.json** - Technical documentation summarisation for technical discussions and implementation details

## Creating Custom Prompts

To create a custom prompt, create a new JSON file in this directory with the following structure:

```json
{
  "name": "Your Prompt Name",
  "description": "Brief description of what this prompt is designed for",
  "prompt": "Your detailed prompt text here. This will be sent to the AI model for summarisation."
}
```

### Prompt Guidelines

- **Name**: Should be unique and descriptive (used as the filename without .json extension)
- **Description**: Brief description shown in the UI dropdown
- **Prompt**: The actual prompt text sent to the AI model

### Example Custom Prompt

```json
{
  "name": "Legal Brief",
  "description": "Summarisation for legal documents and case briefs",
  "prompt": "You are a legal document summariser. Your ONLY purpose is to read legal transcripts and produce structured summaries focusing on key legal points, precedents, and decisions. Be precise and formal. Never invent facts, names, or dates—if information is missing, write \"Unknown.\"\n\nInstructions:\n- Write in Markdown with clear headings\n- Focus on legal precedents, case law, and regulatory requirements\n- Highlight key legal decisions and their implications\n- Include relevant citations and references\n- Never output anything except the summary."
}
```

## Usage

1. Create your custom prompt JSON file in this directory
2. Restart the Blackbox application
3. Your custom prompt will appear in the dropdown menus in both the Auto and Tools tabs
4. Select your custom prompt to use it for summarisation

## File Management

- Custom prompt files are automatically loaded when the application starts
- Files must be valid JSON format
- Invalid files will be skipped with a warning
- Default prompts (meeting.json, dictation.json) cannot be overridden by custom files

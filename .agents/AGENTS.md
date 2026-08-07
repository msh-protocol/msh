# msh-protocol Development Rules

## Feature Workflow
When building new features, you MUST follow this exact workflow:
1. **Branching**: Create a new branch for the feature (e.g., `feat/feature-name`). Do NOT commit directly to `main`.
2. **Documentation**: After building the feature, you MUST update the `docs/` folder to reflect the changes.
3. **Changelog**: Add an entry to the `[Unreleased]` section of `CHANGELOG.md`.
4. **Readme**: Update `README.md` to highlight the new feature or update any affected examples.

## Git Operations
- **NEVER** run `git add`, `git commit`, or `git push` yourself. Always leave these commands for the user to run in their own terminal. You may only run `git checkout` to switch or create branches.

- **HOWEVER**, you MUST explicitly notify the user and tell them when it is time to run these commands (e.g., at the end of a feature or phase) so they can commit and push the changes.


## Command Execution Rule
Whenever you need to run a terminal command, you MUST prepend `msh wrap` to it. 
For example, instead of running `npm run build`, you must run `msh wrap "npm run build" --max-lines 500`.




# QuestLog Habit Tracker

A local-first gamified habit tracker inspired by the Google Sheet version.

## Stack
- **Backend:** Go (`net/http` only, no external Go packages)
- **Frontend:** HTML/CSS/vanilla JavaScript, served by the Go app
- **Persistence:** local JSON file at `data/state.json`

## Run it

From this folder:

```bash
go run .
```

Then open:

```text
http://localhost:8080
```

Your habits, activity, score, and settings persist in `data/state.json`, so stopping and restarting the app will not erase your progress.

## Included in v1
- Create and edit positive or negative habits
- Assign custom XP values, categories, and emoji icons
- Log a habit with a game-style XP animation
- Undo logged actions
- Persistent score and activity history
- Rank ladder based on the original Sheet thresholds
- Progress bar to the next rank
- Import an existing score using **Settings → Starting score**
- Responsive game-style dashboard

## Rank thresholds
Bronze I starts at 0 XP and the ladder continues through Master at 20,000 XP, matching the thresholds visible in the original tracker.

## Data model / future DB migration
All persistent data is isolated behind the Go `Store`. Right now it serializes to JSON. Later, the Store can be swapped for SQLite/Postgres without changing the frontend API.

Good next additions:
1. Daily/weekly streaks and combo multipliers
2. Recurring habit schedules and due-state
3. Custom rank editor / seasons
4. Charts and weekly recaps
5. Achievements/badges
6. SQLite, then hosted Postgres/Supabase when deploying
7. Authentication if multiple users are needed

## Season system and deleting habits
- Each season runs through **December 31, 11:59:59 PM** in the **Go server's local timezone**. On January 1 at midnight, the next request (or server startup) automatically archives the preceding season and starts a new one with **0 XP**, while retaining your habits. The dashboard has a live days/hours/minutes/seconds countdown.
- Previous seasons are stored in `data/state.json` under `archives`, including historical XP, rank, and entries. Click **Rankings** to view archived season summaries. An offline server performs the rollover on its next start; the old season is not lost.
- **Delete habits** with the ✕ on their cards. Deleting a habit does *not* erase XP already logged or its history. Confirm before deletion.
- **Manual reset:** Open **Settings → Reset season XP now**. This archives the current season and resets XP and activity immediately, but leaves habits intact.
- The existing v1 `data/state.json` format is automatically migrated on startup and assigned to the current season without resetting your points. Keep a backup before major changes.

## v3: Past Seasons and scrollable activity
- Select **Past Seasons** in the left sidebar to open a dedicated history page. Choose an archived season to see its final score/rank, earned and lost XP, action count, and historical activity. The history reads existing `archives` from `data/state.json` and needs no database migration.
- **Recent activity** scrolls inside its own fixed-height panel rather than extending the dashboard. All current-season entries are accessible there.
- On narrow screens, navigation remains accessible at the top rather than disappearing.

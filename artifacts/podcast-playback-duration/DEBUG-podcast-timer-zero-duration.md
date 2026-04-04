# Debug: podcast timer shows elapsed / 0:00

## Phase 1: Root Cause Investigation
- **Error:** podcast playback footer can show advancing elapsed time with total duration stuck at `0:00`
- **Reproduction:** start podcast episode playback from a surface carrying an episode object with `Duration == 0`; playback starts and elapsed time advances, but the footer total remains zero
- **Recent changes:** recent search work was unrelated to playback duration; the relevant playback code still initialized episode sessions from the selected episode object
- **Evidence:** runtime logs show episode sessions being created with non-zero track duration, e.g. `trackDuration=9454.367333`, while `internal/app/playback.go` initialized `PlaySessionData.Duration` from `episode.Duration`
- **Data flow trace:** footer total duration comes from `m.player.Duration` → `handlePlaySessionMsg` copies `msg.Session.Duration` into player state → `handlePlayEpisodeCmd` created `PlaySessionData.Duration` from `episode.Duration` instead of the play session metadata/track span

## Phase 2: Pattern Analysis
- **Working example:** book playback already builds `PlaySessionMsg` from session data plus stable item metadata in `buildBookPlaySessionMsg`
- **Differences:** episode playback bypassed a helper and trusted the UI-selected `abs.PodcastEpisode`, which can be partially populated on some entry paths
- **Dependencies:** playback duration must prefer reliable ABS duration data when available, because mpv can report `0` for streaming content

## Phase 3: Hypothesis
- **Hypothesis:** zero total duration is caused by trusting `episode.Duration` from the navigation payload even when the ABS play session has a valid duration source
- **Test:** add a regression where `detail.PlayEpisodeCmd` carries `Duration == 0` but the episode play session returns a non-zero audio track duration
- **Result:** confirmed

## Phase 4: Fix
- **Root cause:** episode playback used incomplete episode metadata as the footer duration source
- **Fix:** resolve podcast duration from the best available source in order: selected episode duration, play-session media metadata duration, then total track span from the play session
- **Test:** `TestPlayEpisodeCmdUsesSessionDurationWhenEpisodeDurationMissing`
- **Verification:** targeted playback regression and full repo test/build pass

## Attempts
- Attempt 1: hypothesis that the footer was initialized from `episode.Duration` instead of play-session duration data → confirmed

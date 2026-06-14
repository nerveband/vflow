package qa

const VideoQAPrompt = `Return JSON only. Evaluate inspectable video-output issues: crop correctness, headroom, eyeline, safe margins, caption occlusion, abrupt crop jumps, wrong speaker on screen, quality regressions, color/exposure/white balance. Do not invent edit decisions. Mark low-confidence observations.`

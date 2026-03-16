import { useEffect, useRef, useState } from "react";
import "./App.css";

type Question = {
  id: number;
  prompt: string;
  hint: string;
  palette: [string, string];
};

type TrackOption = {
  id: string;
  title: string;
  mood: string;
  texture: string;
  bpm: number;
  description: string;
  accent: [string, string];
  frequency: number;
};

type StepResult = {
  questionId: number;
  prompt: string;
  answer: string;
  selectedTrack: TrackOption;
};

const questions: Question[] = [
  {
    id: 1,
    prompt: "この曲の最初の景色を言葉で教えてください。",
    hint: "例: 深夜の高速道路、雨上がりの屋上、朝焼けの港",
    palette: ["#ffb36b", "#f15c80"],
  },
  {
    id: 2,
    prompt: "どんな感情の揺れを入れたいですか？",
    hint: "例: 高揚と不安、静けさの中の熱、やわらかな決意",
    palette: ["#ffd36e", "#ff7a59"],
  },
  {
    id: 3,
    prompt: "リズムやノリの方向性を自由に書いてください。",
    hint: "例: 跳ねる、まっすぐ進む、ゆっくり波打つ、足元が重い",
    palette: ["#91eae4", "#7f7fd5"],
  },
  {
    id: 4,
    prompt: "入れたい音色や楽器の印象はありますか？",
    hint: "例: 粒立つシンセ、湿ったピアノ、荒いドラム、息の近い声",
    palette: ["#c6ffdd", "#fbd786"],
  },
  {
    id: 5,
    prompt: "最後に、この曲が着地する瞬間を描写してください。",
    hint: "例: ふっと光が差す、余韻を残して消える、強く締める",
    palette: ["#a18cd1", "#fbc2eb"],
  },
];

const moods = [
  "Neon Drift",
  "Glass Pulse",
  "Velvet Tide",
  "Afterglow Rail",
  "Static Bloom",
];

const textures = [
  "低音が広がるアンビエント層",
  "粒の細かいアルペジオ",
  "乾いたキックと丸いベース",
  "霞のかかったパッド",
  "近接したボーカルチョップ",
];

const bpmOffsets = [0, 8, -6, 12, 4];
const freqOffsets = [0, 35, 70, 120, 170];

function clampText(text: string, fallback: string) {
  const trimmed = text.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

function buildOptions(question: Question, answer: string): TrackOption[] {
  const seed = clampText(answer, "untitled scene");
  const baseBpm = 82 + (seed.length % 24);
  const baseFrequency = 180 + question.id * 22;

  return Array.from({ length: 5 }, (_, index) => ({
    id: `${question.id}-${index + 1}`,
    title: `${moods[index]} ${question.id}`,
    mood: `${seed.slice(0, 20)}${seed.length > 20 ? "..." : ""}`,
    texture: textures[(index + question.id) % textures.length],
    bpm: Math.max(68, baseBpm + bpmOffsets[index]),
    description: `${seed} をもとにした第${question.id}案。${index + 1}番目は ${textures[index]} を前面に出したラフです。`,
    accent: [
      question.palette[index % 2],
      index % 2 === 0 ? "#101522" : question.palette[(index + 1) % 2],
    ],
    frequency: baseFrequency + freqOffsets[index],
  }));
}

function buildFinalSummary(results: StepResult[]) {
  const selectedTitles = results.map((item) => item.selectedTrack.title).join(" / ");
  const story = results.map((item) => item.answer).join(" -> ");

  return {
    title: `KIRIA Suite ${results.length.toString().padStart(2, "0")}`,
    subtitle: selectedTitles,
    description: `5つの回答と選択されたトラックを重ね、${story} という流れを持つ共同制作曲として完成しました。`,
  };
}

export default function App() {
  const [phase, setPhase] = useState<"intro" | "question" | "complete">("intro");
  const [stepIndex, setStepIndex] = useState(0);
  const [answer, setAnswer] = useState("");
  const [options, setOptions] = useState<TrackOption[]>([]);
  const [results, setResults] = useState<StepResult[]>([]);
  const [playingId, setPlayingId] = useState<string | null>(null);
  const audioRef = useRef<AudioContext | null>(null);
  const oscillatorsRef = useRef<OscillatorNode[]>([]);
  const gainRef = useRef<GainNode | null>(null);

  const currentQuestion = questions[stepIndex];
  const finalSummary = buildFinalSummary(results);

  useEffect(() => {
    return () => {
      stopPreview();
      void audioRef.current?.close();
    };
  }, []);

  function ensureAudioContext() {
    if (!audioRef.current) {
      audioRef.current = new window.AudioContext();
    }

    if (!gainRef.current) {
      gainRef.current = audioRef.current.createGain();
      gainRef.current.gain.value = 0.08;
      gainRef.current.connect(audioRef.current.destination);
    }

    return audioRef.current;
  }

  function stopPreview() {
    for (const oscillator of oscillatorsRef.current) {
      try {
        oscillator.stop();
      } catch {
        // no-op
      }
      oscillator.disconnect();
    }
    oscillatorsRef.current = [];
    setPlayingId(null);
  }

  async function handlePreview(track: TrackOption) {
    if (playingId === track.id) {
      stopPreview();
      return;
    }

    stopPreview();

    const context = ensureAudioContext();
    await context.resume();

    const gain = gainRef.current;
    if (!gain) {
      return;
    }

    const now = context.currentTime;
    const oscillators = [0, 4, 7].map((offset, index) => {
      const oscillator = context.createOscillator();
      const toneGain = context.createGain();

      oscillator.type = index === 0 ? "sine" : index === 1 ? "triangle" : "sawtooth";
      oscillator.frequency.setValueAtTime(track.frequency + offset * 12, now);

      toneGain.gain.setValueAtTime(0.0001, now);
      toneGain.gain.exponentialRampToValueAtTime(0.05 / (index + 1), now + 0.08);
      toneGain.gain.exponentialRampToValueAtTime(0.0001, now + 1.8);

      oscillator.connect(toneGain);
      toneGain.connect(gain);
      oscillator.start(now);
      oscillator.stop(now + 1.82);
      oscillator.onended = () => {
        oscillator.disconnect();
        toneGain.disconnect();
      };

      return oscillator;
    });

    oscillatorsRef.current = oscillators;
    setPlayingId(track.id);

    window.setTimeout(() => {
      setPlayingId((current) => (current === track.id ? null : current));
    }, 1850);
  }

  function handleStart() {
    setPhase("question");
    setStepIndex(0);
    setResults([]);
    setOptions([]);
    setAnswer("");
  }

  function handleGenerateOptions() {
    if (!currentQuestion) {
      return;
    }

    const nextOptions = buildOptions(currentQuestion, answer);
    setOptions(nextOptions);
    stopPreview();
  }

  function handleSelect(track: TrackOption) {
    if (!currentQuestion) {
      return;
    }

    const nextResults = [
      ...results,
      {
        questionId: currentQuestion.id,
        prompt: currentQuestion.prompt,
        answer: clampText(answer, currentQuestion.hint),
        selectedTrack: track,
      },
    ];

    stopPreview();
    setResults(nextResults);
    setAnswer("");
    setOptions([]);

    if (stepIndex === questions.length - 1) {
      setPhase("complete");
      return;
    }

    setStepIndex((current) => current + 1);
  }

  function handleRestart() {
    stopPreview();
    setPhase("intro");
    setStepIndex(0);
    setAnswer("");
    setOptions([]);
    setResults([]);
  }

  return (
    <main className="app-shell">
      <div className="mesh mesh-left" />
      <div className="mesh mesh-right" />

      <section className="hero-panel">
        <p className="eyebrow">Co-Creation Music Generator</p>
        <h1>音楽共同制作生成アプリ</h1>
        <p className="lead">
          言葉で世界観を育て、各ステップで5つのトラック案を試聴しながら、
          最終的な1曲を一緒に組み上げていく体験です。
        </p>

        {phase === "intro" ? (
          <div className="intro-actions">
            <button className="primary-button" onClick={handleStart}>
              スタート
            </button>
          </div>
        ) : (
          <div className="progress-strip">
            {questions.map((question, index) => {
              const state =
                index < results.length
                  ? "done"
                  : index === stepIndex && phase !== "complete"
                    ? "current"
                    : phase === "complete"
                      ? "done"
                      : "todo";

              return (
                <div key={question.id} className={`progress-node progress-${state}`}>
                  <span>{question.id}</span>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {phase === "question" && currentQuestion ? (
        <section className="workspace">
          <div className="question-card">
            <div className="question-meta">
              <span>Question {currentQuestion.id} / 5</span>
              <strong>{currentQuestion.prompt}</strong>
            </div>

            <label className="input-area">
              <span>自由記述</span>
              <textarea
                value={answer}
                onChange={(event) => setAnswer(event.target.value)}
                placeholder={currentQuestion.hint}
                rows={5}
              />
            </label>

            <div className="question-actions">
              <button
                className="primary-button"
                onClick={handleGenerateOptions}
                disabled={answer.trim().length === 0}
              >
                回答からトラック案を生成
              </button>
            </div>
          </div>

          <div className="tracks-panel">
            <div className="tracks-header">
              <h2>トラック候補</h2>
              <p>デモ再生で雰囲気を確認し、1つ選択してください。</p>
            </div>

            <div className="track-grid">
              {options.length === 0 ? (
                <div className="empty-state">
                  <p>回答を入力して「回答からトラック案を生成」を押すと、5つの候補が表示されます。</p>
                </div>
              ) : (
                options.map((track) => (
                  <article key={track.id} className="track-card">
                    <div
                      className="track-image"
                      style={{
                        background: `linear-gradient(135deg, ${track.accent[0]}, ${track.accent[1]})`,
                      }}
                    >
                      <span>{track.title}</span>
                    </div>

                    <div className="track-content">
                      <div className="track-copy">
                        <h3>{track.title}</h3>
                        <p className="track-mood">{track.mood}</p>
                        <p className="track-description">{track.description}</p>
                      </div>

                      <dl className="track-specs">
                        <div>
                          <dt>BPM</dt>
                          <dd>{track.bpm}</dd>
                        </div>
                        <div>
                          <dt>Texture</dt>
                          <dd>{track.texture}</dd>
                        </div>
                      </dl>

                      <div className="track-actions">
                        <button
                          className="secondary-button"
                          onClick={() => void handlePreview(track)}
                        >
                          {playingId === track.id ? "停止" : "デモ再生"}
                        </button>
                        <button
                          className="primary-button"
                          onClick={() => handleSelect(track)}
                        >
                          この案を選択
                        </button>
                      </div>
                    </div>
                  </article>
                ))
              )}
            </div>
          </div>
        </section>
      ) : null}

      {phase === "complete" ? (
        <section className="final-panel">
          <div className="final-hero">
            <p className="eyebrow">Final Output</p>
            <h2>{finalSummary.title}</h2>
            <p className="lead">{finalSummary.description}</p>
          </div>

          <div className="final-track">
            <div className="final-art" />
            <div className="final-copy">
              <h3>最終生成された音楽</h3>
              <p>{finalSummary.subtitle}</p>
              <button
                className="primary-button"
                onClick={() =>
                  void handlePreview({
                    id: "final-preview",
                    title: finalSummary.title,
                    mood: "final",
                    texture: "選択された5案の統合プレビュー",
                    bpm: 108,
                    description: finalSummary.description,
                    accent: ["#ff9966", "#ff5e62"],
                    frequency: 240,
                  })
                }
              >
                {playingId === "final-preview" ? "停止" : "最終曲をデモ再生"}
              </button>
            </div>
          </div>

          <div className="timeline">
            {results.map((item) => (
              <article key={item.questionId} className="timeline-card">
                <span>STEP {item.questionId}</span>
                <h3>{item.prompt}</h3>
                <p>{item.answer}</p>
                <strong>{item.selectedTrack.title}</strong>
              </article>
            ))}
          </div>

          <div className="intro-actions">
            <button className="secondary-button" onClick={handleRestart}>
              もう一度はじめる
            </button>
          </div>
        </section>
      ) : null}
    </main>
  );
}

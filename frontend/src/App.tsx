import { useMemo, useState } from "react";
import "./App.css";

type Question = {
  id: number;
  prompt: string;
  hint: string;
};

type SearchResult = {
  id: number;
  originalFilename: string;
  mimeType: string;
  fileSizeBytes: number;
  embeddingModel: string;
  embeddingDimensions: number;
  similarityScore: number;
  downloadUrl: string;
};

type SearchResponse = {
  query: string;
  results: SearchResult[];
};

type SelectedStep = {
  questionId: number;
  prompt: string;
  answer: string;
  selectedTrack: SearchResult;
};

const questions: Question[] = [
  {
    id: 1,
    prompt: "この曲の最初の景色を言葉で教えてください。",
    hint: "例: 深夜の高速道路、雨上がりの屋上、朝焼けの港",
  },
  {
    id: 2,
    prompt: "どんな感情の揺れを入れたいですか？",
    hint: "例: 高揚と不安、静けさの中の熱、やわらかな決意",
  },
  {
    id: 3,
    prompt: "リズムやノリの方向性を自由に書いてください。",
    hint: "例: 跳ねる、まっすぐ進む、ゆっくり波打つ、足元が重い",
  },
  {
    id: 4,
    prompt: "入れたい音色や楽器の印象はありますか？",
    hint: "例: 粒立つシンセ、湿ったピアノ、荒いドラム、息の近い声",
  },
  {
    id: 5,
    prompt: "最後に、この曲が着地する瞬間を描写してください。",
    hint: "例: ふっと光が差す、余韻を残して消える、強く締める",
  },
];

const gradients: Array<[string, string]> = [
  ["#ffaf7b", "#d76d77"],
  ["#43cea2", "#185a9d"],
  ["#f7971e", "#ffd200"],
  ["#7f7fd5", "#86a8e7"],
  ["#f953c6", "#b91d73"],
];

function buildTrackCopy(track: SearchResult) {
  const name = track.originalFilename.replace(/\.[^.]+$/, "");
  const descriptor =
    track.similarityScore >= 0.9
      ? "かなり近い"
      : track.similarityScore >= 0.8
        ? "近い"
        : "方向性が近い";

  return {
    title: name,
    summary: `${descriptor}候補 / 類似度 ${(track.similarityScore * 100).toFixed(1)}%`,
    texture: `${track.mimeType} / ${(track.fileSizeBytes / 1024).toFixed(1)} KB`,
  };
}

function buildFinalText(steps: SelectedStep[]) {
  return steps.map((step) => step.answer).join(" -> ");
}

export default function App() {
  const [phase, setPhase] = useState<"intro" | "question" | "complete">("intro");
  const [stepIndex, setStepIndex] = useState(0);
  const [answer, setAnswer] = useState("");
  const [results, setResults] = useState<SelectedStep[]>([]);
  const [candidates, setCandidates] = useState<SearchResult[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const currentQuestion = questions[stepIndex];
  const finalText = useMemo(() => buildFinalText(results), [results]);
  const finalTrack = results.length > 0 ? results[results.length - 1].selectedTrack : null;

  function handleStart() {
    setPhase("question");
    setStepIndex(0);
    setAnswer("");
    setResults([]);
    setCandidates([]);
    setError("");
  }

  async function handleSearch() {
    if (!currentQuestion || answer.trim().length === 0) {
      return;
    }

    setIsLoading(true);
    setError("");
    setCandidates([]);

    try {
      const response = await fetch("/api/search/text", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          text: answer.trim(),
          limit: 5,
        }),
      });

      const body = (await response.json()) as SearchResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in body && body.error ? body.error : "検索に失敗しました");
      }

      const nextCandidates = "results" in body ? body.results : [];
      setCandidates(nextCandidates);
      if (nextCandidates.length === 0) {
        setError("一致する音声候補が見つかりませんでした。別の表現で試してください。");
      }
    } catch (fetchError) {
      setError(fetchError instanceof Error ? fetchError.message : "検索に失敗しました");
    } finally {
      setIsLoading(false);
    }
  }

  function handleSelect(track: SearchResult) {
    if (!currentQuestion) {
      return;
    }

    const next = [
      ...results,
      {
        questionId: currentQuestion.id,
        prompt: currentQuestion.prompt,
        answer: answer.trim(),
        selectedTrack: track,
      },
    ];

    setResults(next);
    setAnswer("");
    setCandidates([]);
    setError("");

    if (stepIndex === questions.length - 1) {
      setPhase("complete");
      return;
    }

    setStepIndex((current) => current + 1);
  }

  function handleRestart() {
    setPhase("intro");
    setStepIndex(0);
    setAnswer("");
    setResults([]);
    setCandidates([]);
    setError("");
  }

  return (
    <main className="app-shell">
      <div className="mesh mesh-left" />
      <div className="mesh mesh-right" />

      <section className="hero-panel">
        <p className="eyebrow">Co-Creation Music Generator</p>
        <h1>音楽共同制作生成アプリ</h1>
        <p className="lead">
          フロントで自由記述を送り、バックエンドの音声埋め込み検索から返ってきた候補を再生しながら、
          5段階で曲の方向性を固めていくフローです。
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
                onClick={() => void handleSearch()}
                disabled={answer.trim().length === 0 || isLoading}
              >
                {isLoading ? "検索中..." : "候補を取得"}
              </button>
            </div>
          </div>

          <div className="tracks-panel">
            <div className="tracks-header">
              <h2>トラック候補</h2>
              <p>バックエンドが返した類似音声を再生し、次の質問へ進む1件を選んでください。</p>
            </div>

            {error ? <p className="error-banner">{error}</p> : null}

            <div className="track-grid">
              {candidates.length === 0 ? (
                <div className="empty-state">
                  <p>回答を送信すると、類似度が近い音声候補を最大5件表示します。</p>
                </div>
              ) : (
                candidates.map((track, index) => {
                  const copy = buildTrackCopy(track);
                  const gradient = gradients[index % gradients.length];

                  return (
                    <article key={`${track.id}-${index}`} className="track-card">
                      <div
                        className="track-image"
                        style={{
                          background: `linear-gradient(135deg, ${gradient[0]}, ${gradient[1]})`,
                        }}
                      >
                        <span>{copy.title}</span>
                      </div>

                      <div className="track-content">
                        <div className="track-copy">
                          <h3>{copy.title}</h3>
                          <p className="track-mood">{copy.summary}</p>
                          <p className="track-description">
                            埋め込みモデル: {track.embeddingModel}
                          </p>
                        </div>

                        <dl className="track-specs">
                          <div>
                            <dt>File</dt>
                            <dd>{copy.texture}</dd>
                          </div>
                          <div>
                            <dt>Vector</dt>
                            <dd>{track.embeddingDimensions} dims</dd>
                          </div>
                        </dl>

                        <audio className="audio-player" controls preload="none" src={track.downloadUrl}>
                          お使いのブラウザは audio 要素に対応していません。
                        </audio>

                        <div className="track-actions">
                          <button className="primary-button" onClick={() => handleSelect(track)}>
                            この案を選択
                          </button>
                        </div>
                      </div>
                    </article>
                  );
                })
              )}
            </div>
          </div>
        </section>
      ) : null}

      {phase === "complete" ? (
        <section className="final-panel">
          <div className="final-hero">
            <p className="eyebrow">Final Output</p>
            <h2>最終生成された音楽</h2>
            <p className="lead">
              5つの質問を経て選ばれた候補をもとに、最終案として次の音声を採用します。
            </p>
          </div>

          {finalTrack ? (
            <div className="final-track">
              <div className="final-art" />
              <div className="final-copy">
                <h3>{finalTrack.originalFilename}</h3>
                <p>{finalText}</p>
                <audio className="audio-player large-player" controls preload="none" src={finalTrack.downloadUrl}>
                  お使いのブラウザは audio 要素に対応していません。
                </audio>
              </div>
            </div>
          ) : null}

          <div className="timeline">
            {results.map((item) => (
              <article key={item.questionId} className="timeline-card">
                <span>STEP {item.questionId}</span>
                <h3>{item.prompt}</h3>
                <p>{item.answer}</p>
                <strong>{item.selectedTrack.originalFilename}</strong>
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

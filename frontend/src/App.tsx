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

type GeneratedClip = {
  filename: string;
  mimeType: string;
  fileSizeBytes: number;
  downloadUrl: string;
  indexedAudioId?: number;
  indexedAudioUrl?: string;
};

type MusicGenerationResponse = {
  prompt: string;
  translatedPrompt?: string;
  negativePrompt?: string;
  model: string;
  modelDisplayName?: string;
  clips: GeneratedClip[];
};

type MusicGenerationRequest = {
  prompt: string;
  sampleCount: number;
  selectedAudioIds: number[];
};

type SelectedStep = {
  questionId: number;
  prompt: string;
  answer: string;
  selectedTrack: SearchResult;
};

type QuestionDraft = {
  answer: string;
  candidates: SearchResult[];
  selectedTrackId: number | null;
};

const questions: Question[] = [
  {
    id: 1,
    prompt: "この曲は、どんな瞬間に流れていてほしい？",
    hint: "例: 夜の帰り道、雨の日の部屋、海辺のドライブ、作業中、恋愛の余韻、一人で泣きたいとき",
  },
  {
    id: 2,
    prompt: "その曲に、どんな感情をまとわせたい？",
    hint: "例: うれしい、切ない、浮遊感、エモい、不穏、かわいい、深夜っぽい",
  },
  {
    id: 3,
    prompt: "この曲の空気感をつくる音は？",
    hint: "例: ピアノ、アコースティックギター、シンセ、重低音、ローファイ、きらきらした音",
  },
  {
    id: 4,
    prompt: "感情は、どんなふうに動いていく？",
    hint: "例: ずっと落ち着いたまま、サビで一気に広がる、少しずつ高まる、最後に余韻を残す",
  },
  {
    id: 5,
    prompt: "この曲のメインフレーズは？",
    hint: "例: 相変わらず、愛変わらず / 深夜の高速 D.live",
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

function buildFinalPrompt(steps: SelectedStep[]) {
  return steps.map((step) => step.answer).join(" -> ");
}

async function parseJSONResponse<T>(response: Response): Promise<T | { error?: string }> {
  const text = await response.text();
  if (text.trim().length === 0) {
    return {
      error:
        response.ok
          ? "サーバーから空のレスポンスが返りました。"
          : `サーバー応答が空でした。HTTP ${response.status}`,
    };
  }

  try {
    return JSON.parse(text) as T | { error?: string };
  } catch {
    return {
      error: `サーバー応答の解析に失敗しました。HTTP ${response.status}`,
    };
  }
}

function createQuestionDrafts() {
  return questions.map<QuestionDraft>(() => ({
    answer: "",
    candidates: [],
    selectedTrackId: null,
  }));
}

export default function App() {
  const [phase, setPhase] = useState<"intro" | "question" | "generating" | "complete">("intro");
  const [stepIndex, setStepIndex] = useState(0);
  const [drafts, setDrafts] = useState<QuestionDraft[]>(() => createQuestionDrafts());
  const [results, setResults] = useState<SelectedStep[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState("");
  const [generatedMusic, setGeneratedMusic] = useState<MusicGenerationResponse | null>(null);
  const [generationError, setGenerationError] = useState("");

  const currentQuestion = questions[stepIndex];
  const currentDraft = drafts[stepIndex] ?? { answer: "", candidates: [], selectedTrackId: null };
  const finalPrompt = useMemo(() => buildFinalPrompt(results), [results]);
  const finalTrack = generatedMusic && generatedMusic.clips.length > 0 ? generatedMusic.clips[0] : null;

  function handleStart() {
    setPhase("question");
    setStepIndex(0);
    setDrafts(createQuestionDrafts());
    setResults([]);
    setSearchError("");
    setGeneratedMusic(null);
    setGenerationError("");
  }

  async function handleSearch() {
    if (!currentQuestion || currentDraft.answer.trim().length === 0) {
      return;
    }

    setIsSearching(true);
    setSearchError("");
    setDrafts((current) =>
      current.map((draft, index) =>
        index === stepIndex ? { ...draft, candidates: [], selectedTrackId: null } : draft,
      ),
    );

    try {
      const response = await fetch("/api/search/text", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          text: currentDraft.answer.trim(),
          limit: 5,
        }),
      });

      const body = await parseJSONResponse<SearchResponse>(response);
      if (!response.ok) {
        throw new Error("error" in body && body.error ? body.error : "検索に失敗しました");
      }

      const nextCandidates = "results" in body ? body.results : [];
      setDrafts((current) =>
        current.map((draft, index) =>
          index === stepIndex ? { ...draft, candidates: nextCandidates } : draft,
        ),
      );
      if (nextCandidates.length === 0) {
        setSearchError("一致する音声候補が見つかりませんでした。別の表現で試してください。");
      }
    } catch (error) {
      setSearchError(error instanceof Error ? error.message : "検索に失敗しました");
    } finally {
      setIsSearching(false);
    }
  }

  async function handleSelect(track: SearchResult) {
    if (!currentQuestion || currentDraft.answer.trim().length === 0) {
      return;
    }

    const trimmedAnswer = currentDraft.answer.trim();
    const nextResults = [
      ...results.slice(0, stepIndex),
      {
        questionId: currentQuestion.id,
        prompt: currentQuestion.prompt,
        answer: trimmedAnswer,
        selectedTrack: track,
      },
    ];

    setResults(nextResults);
    setSearchError("");
    setDrafts((current) =>
      current.map((draft, index) =>
        index === stepIndex ? { ...draft, answer: trimmedAnswer, selectedTrackId: track.id } : draft,
      ),
    );

    if (stepIndex < questions.length - 1) {
      setStepIndex((current) => current + 1);
      return;
    }

    setPhase("generating");
    setGenerationError("");
    setGeneratedMusic(null);

    try {
      const prompt = buildFinalPrompt(nextResults);
      const request: MusicGenerationRequest = {
        prompt,
        sampleCount: 4,
        selectedAudioIds: nextResults.map((step) => step.selectedTrack.id),
      };
      const response = await fetch("/api/music/generate", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(request),
      });

      const body = await parseJSONResponse<MusicGenerationResponse>(response);
      if (!response.ok) {
        throw new Error("error" in body && body.error ? body.error : "最終曲の生成に失敗しました");
      }

      const generated = "clips" in body ? body : null;
      if (!generated || generated.clips.length === 0) {
        throw new Error("最終曲の生成結果が空でした");
      }

      setGeneratedMusic(generated);
      setPhase("complete");
    } catch (error) {
      setGenerationError(error instanceof Error ? error.message : "最終曲の生成に失敗しました");
      setPhase("complete");
    }
  }

  function handleBack() {
    if (stepIndex === 0) {
      return;
    }

    const previousIndex = stepIndex - 1;
    setStepIndex(previousIndex);
    setResults((current) => current.slice(0, previousIndex));
    setSearchError("");
  }

  function handleRestart() {
    setPhase("intro");
    setStepIndex(0);
    setDrafts(createQuestionDrafts());
    setResults([]);
    setSearchError("");
    setGeneratedMusic(null);
    setGenerationError("");
  }

  return (
    <main className="app-shell">
      <div className="mesh mesh-left" />
      <div className="mesh mesh-right" />

      <section className="hero-panel">
        <p className="eyebrow">Co-Creation Music Generator</p>
        <h1>Vibe Create Music</h1>
        <p className="lead">
          自由記述を送信するとバックエンドが類似音声を返し、5段階の選択を経たあとに Lyria で最終曲を生成します。
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
                  : index === stepIndex && phase === "question"
                    ? "current"
                    : phase === "generating" || phase === "complete"
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
                value={currentDraft.answer}
                onChange={(event) => {
                  const nextAnswer = event.target.value;
                  setDrafts((current) =>
                    current.map((draft, index) =>
                      index === stepIndex
                        ? {
                            ...draft,
                            answer: nextAnswer,
                            candidates: nextAnswer === draft.answer ? draft.candidates : [],
                            selectedTrackId:
                              nextAnswer === draft.answer ? draft.selectedTrackId : null,
                          }
                        : draft,
                    ),
                  );
                  setSearchError("");
                }}
                placeholder={currentQuestion.hint}
                rows={5}
              />
            </label>

            <div className="question-actions">
              {stepIndex > 0 ? (
                <button className="secondary-button" onClick={handleBack} disabled={isSearching}>
                  前の質問へ
                </button>
              ) : null}
              <button
                className="primary-button"
                onClick={() => void handleSearch()}
                disabled={currentDraft.answer.trim().length === 0 || isSearching}
              >
                {isSearching ? "検索中..." : "候補を取得"}
              </button>
            </div>
          </div>

          <div className="tracks-panel">
            <div className="tracks-header">
              <h2>トラック候補</h2>
              <p>バックエンドが返した類似音声を再生し、次の質問に進む1件を選択してください。</p>
            </div>

            {searchError ? <p className="error-banner">{searchError}</p> : null}

            <div className="track-grid">
              {currentDraft.candidates.length === 0 ? (
                <div className="empty-state">
                  <p>回答を送信すると、類似度が近い音声候補を最大5件表示します。</p>
                </div>
              ) : (
                currentDraft.candidates.map((track, index) => {
                  const copy = buildTrackCopy(track);
                  const gradient = gradients[index % gradients.length];
                  const isSelected = currentDraft.selectedTrackId === track.id;

                  return (
                    <article
                      key={`${track.id}-${index}`}
                      className={`track-card${isSelected ? " track-card-selected" : ""}`}
                    >
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
                          <button className="primary-button" onClick={() => void handleSelect(track)}>
                            {isSelected ? "この案で進む" : "この案を選択"}
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

      {phase === "generating" ? (
        <section className="final-panel">
          <div className="final-hero">
            <p className="eyebrow">Generating</p>
            <h2>Lyria で最終曲を生成しています</h2>
            <p className="lead">
              各質問への回答をもとに 4 本生成し、選択した音楽候補に最も近い 1 本を選びます。
            </p>
          </div>
        </section>
      ) : null}

      {phase === "complete" ? (
        <section className="final-panel">
          <div className="final-hero">
            <p className="eyebrow">Final Output</p>
            <h2>最終生成された音楽</h2>
            <p className="lead">
              Lyria が生成した 4 本から、あなたが選んだ音楽候補に最も近かった最終音源です。
            </p>
          </div>

          {generationError ? <p className="error-banner">{generationError}</p> : null}

          {finalTrack ? (
            <div className="final-track">
              <div className="final-art" />
              <div className="final-copy">
                <h3>最終生成トラック</h3>
                <p>入力プロンプト: {generatedMusic?.prompt ?? finalPrompt}</p>
                <p>生成用プロンプト: {generatedMusic?.translatedPrompt ?? finalPrompt}</p>
                <p>
                  {generatedMusic?.modelDisplayName ?? generatedMusic?.model ?? "Lyria"} /{" "}
                  {(finalTrack.fileSizeBytes / 1024).toFixed(1)} KB
                </p>
                <audio className="audio-player large-player" controls preload="none" src={finalTrack.downloadUrl}>
                  お使いのブラウザは audio 要素に対応していません。
                </audio>
              </div>
            </div>
          ) : (
            <div className="empty-state">
              <p>最終曲の生成に失敗しました。設定を確認して再実行してください。</p>
            </div>
          )}

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

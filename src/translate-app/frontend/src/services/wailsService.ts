import * as Go from '../../wailsjs/go/handler/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { Message } from '@/types/session'
import type { Settings } from '@/types/settings'
import type {
  CreateSessionAndSendRequest,
  CreateSessionAndSendResult,
  FileRequest,
  MessagesPage,
  SendRequest,
} from '@/types/ipc'
import type { FileResult, FileProgress } from '@/types/file'

export const WailsService = {
  getSessions: () => Go.GetSessions(),
  createSessionAndSend: (req: CreateSessionAndSendRequest) =>
    Go.CreateSessionAndSend(req).then((r) => ({
      sessionId: r.sessionId,
      messageId: r.messageId,
    })) as Promise<CreateSessionAndSendResult>,
  renameSession: (id: string, title: string) => Go.RenameSession(id, title),
  updateSessionStatus: (id: string, status: string) => Go.UpdateSessionStatus(id, status),
  getMessages: (sessionId: string, cursor: number, limit: number) =>
    Go.GetMessages(sessionId, cursor, limit) as Promise<MessagesPage>,
  sendMessage: (req: SendRequest) => Go.SendMessage(req),
  openFileDialog: () => Go.OpenFileDialog(),
  readFileInfo: (path: string) => Go.ReadFileInfo(path),
  translateFile: (req: FileRequest) => Go.TranslateFile(req),
  getFileContent: (fileId: string) => Go.GetFileContent(fileId),
  exportMessage: (id: string, format: string) => Go.ExportMessage(id, format),
  exportSession: (id: string, format: string) => Go.ExportSession(id, format),
  exportFile: (fileId: string, format: string) => Go.ExportFile(fileId, format),
  copyTranslation: (messageId: string) => Go.CopyTranslation(messageId),
  getSettings: () => Go.GetSettings() as Promise<Settings>,
  saveSettings: (s: Settings) => Go.SaveSettings(s),
}

export const WailsEvents = {
  onTranslationStart: (cb: (payload: { messageId: string; sessionId: string }) => void) =>
    EventsOn('translation:start', (...a: unknown[]) =>
      cb(a[0] as { messageId: string; sessionId: string }),
    ),
  /** Backend emits raw string chunks. */
  onTranslationChunk: (cb: (chunk: string) => void) =>
    EventsOn('translation:chunk', (...a: unknown[]) => cb(a[0] as string)),
  /** Backend emits the full `Message` object. */
  onTranslationDone: (cb: (message: Message) => void) =>
    EventsOn('translation:done', (...a: unknown[]) => cb(a[0] as Message)),
  /** Backend emits error string. */
  onTranslationError: (cb: (err: string) => void) =>
    EventsOn('translation:error', (...a: unknown[]) => cb(a[0] as string)),
  onFileSource: (cb: (payload: { markdown: string }) => void) =>
    EventsOn('file:source', (...a: unknown[]) => cb(a[0] as { markdown: string })),
  onFileProgress: (cb: (p: FileProgress) => void) =>
    EventsOn('file:progress', (...a: unknown[]) => cb(a[0] as FileProgress)),
  onFileChunkDone: (cb: (c: { chunkIndex: number; text: string }) => void) =>
    EventsOn('file:chunk_done', (...a: unknown[]) => cb(a[0] as { chunkIndex: number; text: string })),
  onFileDone: (cb: (r: FileResult) => void) =>
    EventsOn('file:done', (...a: unknown[]) => cb(a[0] as FileResult)),
  onFileError: (cb: (err: string) => void) =>
    EventsOn('file:error', (...a: unknown[]) => cb(a[0] as string)),
}

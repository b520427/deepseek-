import { useState, useRef } from 'react'
import { X, Upload, FileText } from 'lucide-react'

export default function BatchImportModal({
    show,
    t,
    loading,
    onClose,
    onImport,
}) {
    const [text, setText] = useState('')
    const [fileName, setFileName] = useState('')
    const fileInputRef = useRef(null)

    if (!show) return null

    const handleFileChange = (e) => {
        const file = e.target.files?.[0]
        if (!file) return
        setFileName(file.name)
        const reader = new FileReader()
        reader.onload = (ev) => {
            setText(ev.target.result)
        }
        reader.readAsText(file, 'UTF-8')
    }

    const handleImport = () => {
        if (!text.trim()) return
        onImport(text.trim())
    }

    const handleClose = () => {
        setText('')
        setFileName('')
        onClose()
    }

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="bg-card w-full max-w-lg rounded-xl border border-border shadow-2xl overflow-hidden animate-in zoom-in-95">
                <div className="p-4 border-b border-border flex justify-between items-center">
                    <h3 className="font-semibold flex items-center gap-2">
                        <Upload className="w-4 h-4" />
                        {t('accountManager.batchImportTitle') || '批量导入账号'}
                    </h3>
                    <button onClick={handleClose} className="text-muted-foreground hover:text-foreground">
                        <X className="w-5 h-5" />
                    </button>
                </div>
                <div className="p-6 space-y-4">
                    <div className="bg-muted/50 border border-border rounded-lg p-3 text-xs text-muted-foreground space-y-1">
                        <p className="font-medium text-foreground text-sm">
                            {t('accountManager.batchImportFormatTitle') || '账号文本格式'}
                        </p>
                        <p>{t('accountManager.batchImportFormatDesc') || '每行一个账号，格式：邮箱----密码'}</p>
                        <code className="block bg-card border border-border rounded p-2 mt-1 text-xs font-mono whitespace-pre">
                            {'account1@example.com----password123\naccount2@example.com----mypass456\naccount3@example.com----secret789'}
                        </code>
                        <p className="text-muted-foreground/70 mt-1">
                            {t('accountManager.batchImportNote') || '以 # 开头的行会被忽略，重复邮箱自动跳过'}
                        </p>
                    </div>

                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <label className="text-sm font-medium">
                                {t('accountManager.batchImportPasteOrUpload') || '粘贴账号文本 或 选择文件'}
                            </label>
                            <button
                                onClick={() => fileInputRef.current?.click()}
                                className="flex items-center gap-1.5 text-xs text-primary hover:text-primary/80 transition-colors font-medium"
                            >
                                <FileText className="w-3.5 h-3.5" />
                                {t('accountManager.batchImportSelectFile') || '选择文件'}
                            </button>
                            <input
                                ref={fileInputRef}
                                type="file"
                                accept=".txt,.csv,text/plain"
                                onChange={handleFileChange}
                                className="hidden"
                            />
                        </div>
                        {fileName && (
                            <div className="text-xs text-emerald-500 mb-2">
                                ✓ {t('accountManager.batchImportFileLoaded') || '已加载文件'}: {fileName}
                            </div>
                        )}
                        <textarea
                            className="w-full h-48 input-field font-mono text-xs resize-y"
                            placeholder={'account1@example.com----password123\naccount2@example.com----mypass456'}
                            value={text}
                            onChange={e => setText(e.target.value)}
                        />
                    </div>

                    <div className="flex justify-end gap-2 pt-2">
                        <button
                            onClick={handleClose}
                            className="px-4 py-2 rounded-lg border border-border hover:bg-secondary transition-colors text-sm font-medium"
                        >
                            {t('actions.cancel') || '取消'}
                        </button>
                        <button
                            onClick={handleImport}
                            disabled={loading || !text.trim()}
                            className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors text-sm font-medium disabled:opacity-50 flex items-center gap-2"
                        >
                            {loading ? (
                                <span className="animate-spin">⟳</span>
                            ) : (
                                <Upload className="w-4 h-4" />
                            )}
                            {t('accountManager.batchImportAction') || '批量导入'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}

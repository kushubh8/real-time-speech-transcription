'use client';

import { useEffect, useRef, useState } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';

function escapeHtml(unsafeText: string): string {
  return unsafeText
    .replaceAll(/&/g, '&amp;')
    .replaceAll(/</g, '&lt;')
    .replaceAll(/>/g, '&gt;')
    .replaceAll(/"/g, '&quot;')
    .replaceAll(/'/g, '&#039;');
}

export default function TipTapEditor() {
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage] = useState<string | null>(null);
  // Tracks the length of the current interim (tentative) text so we can
  // replace it in-place when the next interim or the final result arrives.
  const interimLengthRef = useRef(0);

  const editor = useEditor({
    extensions: [StarterKit],
    content: '<p></p>',
    editable: true,
    immediatelyRender: false,
    autofocus: true,
    editorProps: {
      attributes: {
        class: 'tiptap-editor',
      },
    },
  });

  // Removed backend fetch. Content will come from WS via ptt:recognized

  useEffect(() => {
    if (!editor) return;
    const handler = (e: Event) => {
      const custom = e as CustomEvent<{ text: string; isFinal: boolean }>;
      const { text, isFinal } = custom.detail ?? {};
      if (!text) return;

      // Delete the previous interim text (if any), then insert the new text.
      // For interim results we leave interimLengthRef set so the next result
      // can overwrite it. For final results we clear it — the text is committed.
      if (interimLengthRef.current > 0) {
        editor
          .chain()
          .focus()
          .deleteRange({
            from: editor.state.selection.anchor - interimLengthRef.current,
            to: editor.state.selection.anchor,
          })
          .run();
      }

      const insert = isFinal ? `${text} ` : text;
      editor.chain().focus().insertContent(insert).run();
      interimLengthRef.current = isFinal ? 0 : insert.length;
    };
    window.addEventListener('ptt:recognized', handler as EventListener);
    return () => window.removeEventListener('ptt:recognized', handler as EventListener);
  }, [editor]);

  const [hasInteracted, setHasInteracted] = useState(false);

  useEffect(() => {
    if (!editor) return;
    const handleUpdate = () => {
      if (!hasInteracted && !editor.isEmpty) setHasInteracted(true);
    };
    editor.on('update', handleUpdate);
    return () => {
      editor.off('update', handleUpdate);
    };
  }, [editor, hasInteracted]);

  return (
    <div className='grid gap-2 h-full'>
      {errorMessage && (
        <div className='text-[#b91c1c] text-sm'>
          Error: {errorMessage}
        </div>
      )}
      <div
        className='relative border border-1 border-[#e5e7eb] rounded-md p-3 min-h-[180px]'
        style={{
          background: isLoading ? '#fafafa' : 'white',
        }}
      >
        {!hasInteracted && editor?.isEmpty && (
          <span
            className='absolute top-3 left-3 text-[#9ca3af] pointer-events-none select-none'
          >
            Start Recording....
          </span>
        )}
        <EditorContent editor={editor} />
      </div>
    </div>
  );
}


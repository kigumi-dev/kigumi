package lsp

import (
	"context"
	"encoding/json"
	stderrors "errors"

	"go.lsp.dev/protocol"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/version"
)

func (s *Server) Initialize(ctx context.Context, p *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	if s.compiler.Cwd == "" {
		return nil, stderrors.New("lsp: CompilerOptions.Cwd is required; build the server with WithPackageCache")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roots = workspaceRoots(p)
	var opts struct {
		StdRoot   string `json:"stdRoot"`
		Allocator string `json:"allocator"`
		Profile   string `json:"profile"`
	}
	if json.Unmarshal([]byte(p.InitializationOptions), &opts) == nil {
		if opts.StdRoot != "" {
			s.stdRoot = opts.StdRoot
		}
		if s.target.OS == "" {
			s.target = buildconfig.HostTarget()
		}
		if opts.Allocator == "none" {
			s.target.Allocator = "none"
		}
		if opts.Profile == "recoverable-only" {
			s.target.Profile = "recoverable-only"
		}
	}
	if s.stdRoot == "" {
		s.stdRoot = findStd(s.primaryRoot())
	}
	// Locations are file URIs, so a std root given relative to the cwd
	// (`kigumi lsp` finds ./std) must become absolute.
	if s.stdRoot != "" {
		if abs, err := s.compiler.Abs(s.stdRoot); err == nil {
			s.stdRoot = abs
		}
	}
	if ws := p.Capabilities.Workspace; ws != nil && ws.WorkspaceEdit != nil && ws.WorkspaceEdit.DocumentChanges != nil {
		s.documentChanges = *ws.WorkspaceEdit.DocumentChanges
	}
	s.logf(ctx, protocol.MessageTypeInfo, "kigumi lsp: workspace %v, std %q", s.roots, s.stdRoot)
	if s.stdRoot == "" {
		s.logf(ctx, protocol.MessageTypeWarning, "std stubs not found: set kigumi.stdRoot or $KIGUMI_STD, or keep std/ in the workspace; std imports will fail to resolve")
	}
	prepareRename, foldersSupported := true, true
	caps := protocol.ServerCapabilities{
		TextDocumentSync:        protocol.TextDocumentSyncKindFull,
		HoverProvider:           protocol.Boolean(true),
		DefinitionProvider:      protocol.Boolean(true),
		ImplementationProvider:  protocol.Boolean(true),
		ReferencesProvider:      protocol.Boolean(true),
		WorkspaceSymbolProvider: protocol.Boolean(true),
		CodeActionProvider: &protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{
			protocol.CodeActionKindQuickFix,
			protocol.CodeActionKindSourceOrganizeImports,
		}},
		RenameProvider:             &protocol.RenameOptions{PrepareProvider: &prepareRename},
		DocumentFormattingProvider: protocol.Boolean(true),
		// No DocumentRangeFormattingProvider: Formatting always reformats
		// the whole file, so range support would promise a behavior never given.
		DocumentSymbolProvider:    protocol.Boolean(true),
		CompletionProvider:        &protocol.CompletionOptions{TriggerCharacters: []string{"."}},
		SignatureHelpProvider:     &protocol.SignatureHelpOptions{TriggerCharacters: []string{"(", ","}},
		DocumentHighlightProvider: protocol.Boolean(true),
		FoldingRangeProvider:      protocol.Boolean(true),
		SemanticTokensProvider: &protocol.SemanticTokensOptions{
			Legend: protocol.SemanticTokensLegend{TokenTypes: semanticTokenTypes, TokenModifiers: semanticTokenModifiers},
			Full:   protocol.Boolean(true),
		},
		Workspace: &protocol.WorkspaceOptions{
			WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{Supported: &foldersSupported},
		},
	}
	return &protocol.InitializeResult{Capabilities: caps, ServerInfo: protocol.ServerInfo{Name: "kigumi", Version: protocol.NewOptional(version.Release)}}, nil
}

// workspaceRoots falls back to the deprecated single-root fields in spec
// order, so a client with no workspaceFolders support still gets analyzed.
func workspaceRoots(p *protocol.InitializeParams) []string {
	if folders, ok := p.WorkspaceFolders.Get(); ok && len(folders) > 0 {
		roots := make([]string, len(folders))
		for i, f := range folders {
			roots[i] = uriToPath(f.URI)
		}
		return roots
	}
	if p.RootURI != nil {
		return []string{uriToPath(*p.RootURI)}
	}
	if rp, ok := p.RootPath.Get(); ok && rp != "" {
		return []string{rp}
	}
	return nil
}

func (s *Server) primaryRoot() string {
	if len(s.roots) == 0 {
		return ""
	}
	return s.roots[0]
}

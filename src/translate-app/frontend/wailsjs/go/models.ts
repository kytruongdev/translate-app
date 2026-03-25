export namespace bridge {
	
	export class CreateSessionAndSendRequest {
	    title?: string;
	    content: string;
	    displayMode: string;
	    sourceLang: string;
	    targetLang: string;
	    style?: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateSessionAndSendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.content = source["content"];
	        this.displayMode = source["displayMode"];
	        this.sourceLang = source["sourceLang"];
	        this.targetLang = source["targetLang"];
	        this.style = source["style"];
	    }
	}
	export class CreateSessionAndSendResult {
	    sessionId: string;
	    messageId: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateSessionAndSendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.messageId = source["messageId"];
	    }
	}
	export class FileContent {
	    sourceMarkdown: string;
	    translatedMarkdown: string;
	
	    static createFrom(source: any = {}) {
	        return new FileContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceMarkdown = source["sourceMarkdown"];
	        this.translatedMarkdown = source["translatedMarkdown"];
	    }
	}
	export class FileInfo {
	    name: string;
	    type: string;
	    fileSize: number;
	    pageCount?: number;
	    charCount: number;
	    isScanned?: boolean;
	    estimatedChunks: number;
	    estimatedMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.fileSize = source["fileSize"];
	        this.pageCount = source["pageCount"];
	        this.charCount = source["charCount"];
	        this.isScanned = source["isScanned"];
	        this.estimatedChunks = source["estimatedChunks"];
	        this.estimatedMinutes = source["estimatedMinutes"];
	    }
	}
	export class FileRequest {
	    sessionId: string;
	    filePath: string;
	    targetLang?: string;
	    style?: string;
	    provider?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.filePath = source["filePath"];
	        this.targetLang = source["targetLang"];
	        this.style = source["style"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	    }
	}
	export class MessagesPage {
	    messages: model.Message[];
	    nextCursor: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MessagesPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], model.Message);
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SendRequest {
	    sessionId: string;
	    content: string;
	    displayMode: string;
	    sourceLang: string;
	    targetLang: string;
	    style?: string;
	    originalMessageId?: string;
	    provider?: string;
	    model?: string;
	    fileId?: string;
	    fileDisplayContent?: string;
	
	    static createFrom(source: any = {}) {
	        return new SendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.content = source["content"];
	        this.displayMode = source["displayMode"];
	        this.sourceLang = source["sourceLang"];
	        this.targetLang = source["targetLang"];
	        this.style = source["style"];
	        this.originalMessageId = source["originalMessageId"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.fileId = source["fileId"];
	        this.fileDisplayContent = source["fileDisplayContent"];
	    }
	}

}

export namespace model {
	
	export class Message {
	    id: string;
	    sessionId: string;
	    role: string;
	    displayOrder: number;
	    displayMode: string;
	    originalContent: string;
	    translatedContent: string;
	    fileId?: string;
	    sourceLang: string;
	    targetLang: string;
	    style: string;
	    modelUsed: string;
	    originalMessageId?: string;
	    tokens: number;
	    fileSize: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.role = source["role"];
	        this.displayOrder = source["displayOrder"];
	        this.displayMode = source["displayMode"];
	        this.originalContent = source["originalContent"];
	        this.translatedContent = source["translatedContent"];
	        this.fileId = source["fileId"];
	        this.sourceLang = source["sourceLang"];
	        this.targetLang = source["targetLang"];
	        this.style = source["style"];
	        this.modelUsed = source["modelUsed"];
	        this.originalMessageId = source["originalMessageId"];
	        this.tokens = source["tokens"];
	        this.fileSize = source["fileSize"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Session {
	    id: string;
	    title: string;
	    status: string;
	    targetLang: string;
	    style: string;
	    model: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.targetLang = source["targetLang"];
	        this.style = source["style"];
	        this.model = source["model"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Settings {
	    theme: string;
	    activeProvider: string;
	    activeModel: string;
	    defaultStyle: string;
	    lastTargetLang: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.activeProvider = source["activeProvider"];
	        this.activeModel = source["activeModel"];
	        this.defaultStyle = source["defaultStyle"];
	        this.lastTargetLang = source["lastTargetLang"];
	    }
	}

}


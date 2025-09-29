export namespace db {
	
	export class Recording {
	    id: number;
	    filename: string;
	    display_name?: string;
	    file_path: string;
	    file_size: number;
	    duration_seconds?: number;
	    sample_rate: number;
	    channels: number;
	    bits_per_sample: number;
	    audio_format: string;
	    recording_mode: string;
	    with_microphone: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    recorded_at?: any;
	    notes?: string;
	    tags?: string;
	    audio_data?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Recording(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.display_name = source["display_name"];
	        this.file_path = source["file_path"];
	        this.file_size = source["file_size"];
	        this.duration_seconds = source["duration_seconds"];
	        this.sample_rate = source["sample_rate"];
	        this.channels = source["channels"];
	        this.bits_per_sample = source["bits_per_sample"];
	        this.audio_format = source["audio_format"];
	        this.recording_mode = source["recording_mode"];
	        this.with_microphone = source["with_microphone"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.recorded_at = this.convertValues(source["recorded_at"], null);
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.audio_data = source["audio_data"];
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
	export class RecordingWithDetails {
	    id: number;
	    filename: string;
	    display_name?: string;
	    file_path: string;
	    file_size: number;
	    duration_seconds?: number;
	    sample_rate: number;
	    channels: number;
	    bits_per_sample: number;
	    audio_format: string;
	    recording_mode: string;
	    with_microphone: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    recorded_at?: any;
	    notes?: string;
	    tags?: string;
	    audio_data?: number[];
	    transcript_id?: number;
	    transcript_content?: string;
	    transcript_model?: string;
	    confidence_score?: number;
	    // Go type: time
	    transcribed_at?: any;
	    summary_id?: number;
	    summary_content?: string;
	    summary_type?: string;
	    summary_model?: string;
	    // Go type: time
	    summarized_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new RecordingWithDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.display_name = source["display_name"];
	        this.file_path = source["file_path"];
	        this.file_size = source["file_size"];
	        this.duration_seconds = source["duration_seconds"];
	        this.sample_rate = source["sample_rate"];
	        this.channels = source["channels"];
	        this.bits_per_sample = source["bits_per_sample"];
	        this.audio_format = source["audio_format"];
	        this.recording_mode = source["recording_mode"];
	        this.with_microphone = source["with_microphone"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.recorded_at = this.convertValues(source["recorded_at"], null);
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.audio_data = source["audio_data"];
	        this.transcript_id = source["transcript_id"];
	        this.transcript_content = source["transcript_content"];
	        this.transcript_model = source["transcript_model"];
	        this.confidence_score = source["confidence_score"];
	        this.transcribed_at = this.convertValues(source["transcribed_at"], null);
	        this.summary_id = source["summary_id"];
	        this.summary_content = source["summary_content"];
	        this.summary_type = source["summary_type"];
	        this.summary_model = source["summary_model"];
	        this.summarized_at = this.convertValues(source["summarized_at"], null);
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
	export class RecordingWithTranscript {
	    id: number;
	    filename: string;
	    display_name: string;
	    file_path: string;
	    duration_seconds: number;
	    // Go type: time
	    recorded_at: any;
	    notes: string;
	    tags: string;
	    transcript_id: number;
	    transcript_content: string;
	    transcript_model: string;
	    confidence_score: number;
	    // Go type: time
	    transcript_created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new RecordingWithTranscript(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.display_name = source["display_name"];
	        this.file_path = source["file_path"];
	        this.duration_seconds = source["duration_seconds"];
	        this.recorded_at = this.convertValues(source["recorded_at"], null);
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.transcript_id = source["transcript_id"];
	        this.transcript_content = source["transcript_content"];
	        this.transcript_model = source["transcript_model"];
	        this.confidence_score = source["confidence_score"];
	        this.transcript_created_at = this.convertValues(source["transcript_created_at"], null);
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
	export class Summary {
	    id: number;
	    transcript_id: number;
	    content: string;
	    summary_type: string;
	    model_used: string;
	    temperature?: number;
	    prompt_used: string;
	    prompt_id?: number;
	    processing_time_seconds?: number;
	    api_endpoint?: string;
	    local_model_path?: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.transcript_id = source["transcript_id"];
	        this.content = source["content"];
	        this.summary_type = source["summary_type"];
	        this.model_used = source["model_used"];
	        this.temperature = source["temperature"];
	        this.prompt_used = source["prompt_used"];
	        this.prompt_id = source["prompt_id"];
	        this.processing_time_seconds = source["processing_time_seconds"];
	        this.api_endpoint = source["api_endpoint"];
	        this.local_model_path = source["local_model_path"];
	        this.created_at = this.convertValues(source["created_at"], null);
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
	export class Transcript {
	    id: number;
	    recording_id: number;
	    content: string;
	    confidence_score?: number;
	    model_used: string;
	    language: string;
	    processing_time_seconds?: number;
	    whisper_version?: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Transcript(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.recording_id = source["recording_id"];
	        this.content = source["content"];
	        this.confidence_score = source["confidence_score"];
	        this.model_used = source["model_used"];
	        this.language = source["language"];
	        this.processing_time_seconds = source["processing_time_seconds"];
	        this.whisper_version = source["whisper_version"];
	        this.created_at = this.convertValues(source["created_at"], null);
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

}

export namespace ui {
	
	export class PromptConfig {
	    name: string;
	    description: string;
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.prompt = source["prompt"];
	    }
	}
	export class UISettings {
	    out_dir: string;
	    database_path: string;
	    enable_file_backups: boolean;
	    use_local_ai: boolean;
	    llama_temp: number;
	    llama_context: number;
	    llama_model: string;
	    llama_api_key: string;
	
	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.out_dir = source["out_dir"];
	        this.database_path = source["database_path"];
	        this.enable_file_backups = source["enable_file_backups"];
	        this.use_local_ai = source["use_local_ai"];
	        this.llama_temp = source["llama_temp"];
	        this.llama_context = source["llama_context"];
	        this.llama_model = source["llama_model"];
	        this.llama_api_key = source["llama_api_key"];
	    }
	}

}


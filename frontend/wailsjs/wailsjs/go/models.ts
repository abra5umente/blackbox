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


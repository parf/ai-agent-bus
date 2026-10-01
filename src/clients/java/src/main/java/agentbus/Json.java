package agentbus;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.core.StreamReadFeature;

import java.io.IOException;
import java.io.StringWriter;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * JSON through jackson-core's strict streaming parser and generator, as
 * plain values: objects are {@code Map<String,Object>} in order, arrays
 * {@code List<Object>}, integers {@code Long} or {@code BigInteger} when larger,
 * other numbers {@code BigDecimal}, so nothing is rounded on the way in. A
 * parsed tree is unmodifiable. Leading zeros, bare decimal points, raw control
 * characters, NaN, comments, duplicate keys and trailing text are refused.
 */
public final class Json {
    private Json() {}

    private static final JsonFactory FACTORY = JsonFactory.builder()
            .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
            .build();

    /** JSON text written into another value as it is, not as a string. Checked to be JSON. */
    public record Raw(String text) {
        public Raw {
            parse(text);
        }
    }

    public static Object parse(String s) {
        try (JsonParser p = FACTORY.createParser(s)) {
            JsonToken t = p.nextToken();
            if (t == null) throw new IllegalArgumentException("no JSON value");
            Object v = value(p, t);
            if (p.nextToken() != null) throw new IllegalArgumentException("trailing text after the JSON value");
            return v;
        } catch (IOException e) {
            throw new IllegalArgumentException("not JSON: " + (e instanceof com.fasterxml.jackson.core.JsonProcessingException j ? j.getOriginalMessage() : e.getMessage()), e);
        }
    }

    @SuppressWarnings("unchecked")
    static Map<String, Object> object(String s) {
        Object v = parse(s);
        if (!(v instanceof Map)) throw new IllegalArgumentException("not a JSON object");
        return (Map<String, Object>) v;
    }

    private static Object value(JsonParser p, JsonToken t) throws IOException {
        switch (t) {
            case START_OBJECT -> {
                Map<String, Object> m = new LinkedHashMap<>();
                for (JsonToken k = p.nextToken(); k != JsonToken.END_OBJECT; k = p.nextToken()) {
                    String name = p.currentName();
                    m.put(name, value(p, p.nextToken()));
                }
                return Collections.unmodifiableMap(m);
            }
            case START_ARRAY -> {
                List<Object> l = new ArrayList<>();
                for (JsonToken x = p.nextToken(); x != JsonToken.END_ARRAY; x = p.nextToken()) l.add(value(p, x));
                return Collections.unmodifiableList(l);
            }
            case VALUE_STRING -> { return p.getText(); }
            case VALUE_NUMBER_INT -> {
                return p.getNumberType() == JsonParser.NumberType.BIG_INTEGER ? p.getBigIntegerValue() : (Object) p.getLongValue();
            }
            case VALUE_NUMBER_FLOAT -> { return p.getDecimalValue(); }
            case VALUE_TRUE -> { return Boolean.TRUE; }
            case VALUE_FALSE -> { return Boolean.FALSE; }
            case VALUE_NULL -> { return null; }
            default -> throw new IllegalArgumentException("unexpected JSON token " + t);
        }
    }

    public static String write(Object v) {
        StringWriter w = new StringWriter();
        try (JsonGenerator g = FACTORY.createGenerator(w)) {
            write(g, v);
        } catch (IOException e) {
            throw new IllegalArgumentException("cannot write JSON: " + e.getMessage(), e);
        }
        return w.toString();
    }

    private static void write(JsonGenerator g, Object v) throws IOException {
        switch (v) {
            case null -> g.writeNull();
            case Raw r -> g.writeRawValue(r.text());
            case String s -> g.writeString(s);
            case Boolean b -> g.writeBoolean(b);
            case Integer x -> g.writeNumber(x);
            case Long x -> g.writeNumber(x);
            case BigInteger x -> g.writeNumber(x);
            case BigDecimal x -> g.writeNumber(x);
            case Double x -> { finite(x); g.writeNumber(x); }
            case Float x -> { finite(x); g.writeNumber(x); }
            case Short x -> g.writeNumber(x);
            case Byte x -> g.writeNumber(x);
            case Map<?, ?> m -> {
                g.writeStartObject();
                for (Map.Entry<?, ?> e : m.entrySet()) {
                    g.writeFieldName(String.valueOf(e.getKey()));
                    write(g, e.getValue());
                }
                g.writeEndObject();
            }
            case Iterable<?> l -> {
                g.writeStartArray();
                for (Object x : l) write(g, x);
                g.writeEndArray();
            }
            default -> throw new IllegalArgumentException("no JSON for " + v.getClass().getName());
        }
    }

    private static void finite(double d) {
        if (Double.isNaN(d) || Double.isInfinite(d)) throw new IllegalArgumentException("JSON has no " + d);
    }
}

module Generic exposing (Value(..), dictToPairs, fromJson, toJson)

import Dict
import Json.Decode as Decode
import Json.Encode as Encode
import List
import OrderedDict as Od


{-| Our generic value containing all possible types we may encounter in json blob
-}
type Value
    = Num Float
    | Txt String
    | Bln Bool
    | Nil
    | Dct (Od.OrderedDict String Value)
    | Lst (List Value)


{-| encode our generic value into json
-}
toJson : Value -> Encode.Value
toJson value =
    case value of
        Num num ->
            Encode.float num

        Txt text ->
            Encode.string text

        Bln fact ->
            Encode.bool fact

        Lst list ->
            Encode.list toJson list

        Dct dict ->
            let
                convert ( key, val ) =
                    ( key, toJson val )

                pairs =
                    dict
                        |> dictToPairs
                        |> List.reverse
                        |> List.map convert
            in
            Encode.object pairs

        Nil ->
            Encode.null


{-| decode our generic value from json
-}
fromJson : Decode.Decoder Value
fromJson =
    Decode.oneOf
        [ numDecoder
        , blnDecoder
        , txtDecoder
        , nilDecoder
        , lstDecoder
        , dctDecoder
        ]



-- num decoding


numDecoder : Decode.Decoder Value
numDecoder =
    Decode.map Num Decode.float



-- text decoding


txtDecoder : Decode.Decoder Value
txtDecoder =
    Decode.map Txt Decode.string



-- boolean decoding


blnDecoder : Decode.Decoder Value
blnDecoder =
    Decode.map Bln Decode.bool



-- nil decoding


nilDecoder : Decode.Decoder Value
nilDecoder =
    Decode.null Nil



-- list decoding


{-| decode list of Value(s)

best to read from the bottom

1.  first we declare reference to ourselves in lazy way to prevent circular trouble
2.  then we decode the list using that decoder from 1.
3.  finally we map Lst on that list thereby returning a decoder that is Decoder Value

-}
lstDecoder : Decode.Decoder Value
lstDecoder =
    Decode.map Lst <|
        Decode.list <|
            Decode.lazy (\_ -> fromJson)



-- dict decoding


{-| decode nested object of String keys and Value values

best read from the bottom

1.  lazy decoder fro value
2.  decoder to break down nested object to pairs
3.  Decode.Map to construct the ordered dict from pairs
4.  Decode.Map with Dct to construct a decoder that returns Decoder.Decoder Value

-}
dctDecoder : Decode.Decoder Value
dctDecoder =
    Decode.map Dct <|
        Decode.map dictFromPairs <|
            Decode.keyValuePairs <|
                Decode.lazy (\_ -> fromJson)



-- utils section mainly to help with OrderedDict storing String Value values


{-| translate list of pairs into OrderedDict
-}
dictFromPairs : List ( String, Value ) -> Od.OrderedDict String Value
dictFromPairs pairs =
    List.foldl (\( key, value ) dict -> Od.insert key value dict) Od.empty pairs


{-| translate OrderedDict to key value pairs while preserving the order
-}
dictToPairs : Od.OrderedDict String Value -> List ( String, Value )
dictToPairs dict =
    let
        func key =
            ( key
            , Maybe.withDefault Nil <|
                Dict.get key dict.dict
            )
    in
    List.map func dict.order
